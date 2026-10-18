//go:build darwin && (amd64 || arm64)

package coreaudio

import (
	"fmt"
	"math"
	"sync/atomic"
	"time"
	"unsafe"

	"m31labs.dev/tymbal/internal/driver"
	"m31labs.dev/tymbal/internal/ring"
	"m31labs.dev/tymbal/internal/rt"
)

// audioBufferList matches one interleaved AudioBuffer in AudioBufferList.
type audioBufferList struct {
	numberBuffers  uint32
	_              uint32
	numberChannels uint32
	dataByteSize   uint32
	data           uintptr
}

// captureCtrl and its buffers live outside the Go heap. The foreign callback
// pulls input with AudioUnitRender, then publishes complete PCM frames. Go is
// the sole consumer. On overflow the producer pauses until Recover discards
// the stale queue, so old and new samples cannot acquire invented continuity.
type captureCtrl struct {
	head                                       uint64
	_                                          [56]byte
	tail                                       uint64
	_                                          [56]byte
	mask, frameBytes                           uint64
	data, unit, render, memcpy, memset, signal uintptr
	sem                                        uint64
	sigmask                                    uintptr
	sigset, masked                             uint64
	maxFrames                                  uint64
	buffer                                     audioBufferList
	scratch                                    uintptr
	actionFlags                                uint64
	meta                                       uintptr
	metaHead                                   uint64
	paused, overflows, droppedFrames           uint64
	status                                     int64
}

func captureInput()

var captureABI0 uintptr

type captureSpan struct {
	start, end, sampleTime, hostTime, flags uint64
}

var errInputClock = fmt.Errorf("%w: coreaudio input clock changed; reopen the stream", driver.ErrLost)

type inputCapture struct {
	arena         *ring.Arena
	c             *captureCtrl
	data, staging []byte
	period        uint64
	clock         hostClock
	spans         []captureSpan
	metaTail      uint64
	pending       driver.ClockSample
	failure       captureError
	lastEnd       float64
	clockKnown    bool
}

type captureError struct{ status int64 }

func (e *captureError) Error() string {
	return fmt.Sprintf("coreaudio: input render failed: %s", describeStatus(int32(e.status)))
}

type hostClock struct {
	numer, denom uint64
	offset       int64
}

func newHostClock() hostClock {
	var numer, denom uint32
	if captureMachTimebase(&numer, &denom) != 0 || numer == 0 || denom == 0 {
		return hostClock{}
	}
	c := hostClock{numer: uint64(numer), denom: uint64(denom)}
	best := int64(math.MaxInt64)
	for i := 0; i < 8; i++ {
		before := rt.Now()
		ticks := captureMachTime()
		after := rt.Now()
		if span := after - before; span < best {
			best = span
			c.offset = before + span/2 - (hostClock{numer: c.numer, denom: c.denom}).nano(ticks)
		}
	}
	return c
}

// nano converts the Mach hardware timestamp into rt.Now's process-local
// monotonic domain. The quotient form avoids overflowing ticks*numer.
//
//tymbal:rt
func (c hostClock) nano(ticks uint64) int64 {
	if c.denom == 0 {
		return 0
	}
	return int64(ticks/c.denom*c.numer+ticks%c.denom*c.numer/c.denom) + c.offset
}

func newInputCapture(channels, period, capacity, maxFrames int, sem uint32) (*inputCapture, error) {
	if channels <= 0 || channels > maxChannels || period <= 0 || capacity < period || maxFrames <= 0 || maxFrames > maxRingFrames {
		return nil, errRingGeometry
	}
	capacity = nextPow2(capacity)
	frameBytes := channels * bytesPerSample
	if capacity > maxRingFrames || (capacity+maxFrames)*frameBytes+capacity*int(unsafe.Sizeof(captureSpan{})) > maxRingBytes {
		return nil, errRingGeometry
	}
	ctrlSize := int(unsafe.Sizeof(captureCtrl{}))
	arena, err := ring.NewArena(ctrlSize + capacity*frameBytes + maxFrames*frameBytes + capacity*int(unsafe.Sizeof(captureSpan{})) + 256)
	if err != nil {
		return nil, err
	}
	_ = arena.Lock()
	control := arena.Take(ctrlSize, 64)
	data := arena.Take(capacity*frameBytes, 64)
	scratch := arena.Take(maxFrames*frameBytes, 64)
	meta := arena.Take(capacity*int(unsafe.Sizeof(captureSpan{})), 64)
	c := (*captureCtrl)(unsafe.Pointer(&control[0]))
	c.mask, c.frameBytes = uint64(capacity-1), uint64(frameBytes)
	c.data, c.render, c.memcpy = uintptr(unsafe.Pointer(&data[0])), fnCaptureAudioUnitRender, fnmemcpy
	c.signal, c.sem, c.memset = fnsemaphore_signal, uint64(sem), fnmemset
	c.scratch = uintptr(unsafe.Pointer(&scratch[0]))
	c.meta = uintptr(unsafe.Pointer(&meta[0]))
	c.sigmask, c.sigset = fnpthread_sigmask, uint64(blockedSignals)
	c.maxFrames = uint64(maxFrames)
	c.buffer = audioBufferList{numberBuffers: 1, numberChannels: uint32(channels), dataByteSize: uint32(len(scratch)), data: uintptr(unsafe.Pointer(&scratch[0]))}
	return &inputCapture{arena: arena, c: c, data: data, staging: make([]byte, period*frameBytes), period: uint64(period), clock: newHostClock(), spans: unsafe.Slice((*captureSpan)(unsafe.Pointer(&meta[0])), capacity)}, nil
}

//tymbal:rt
func (s *inputCapture) waitError() error {
	if status := atomic.LoadInt64(&s.c.status); status != 0 {
		s.failure.status = status
		return &s.failure
	}
	if atomic.LoadUint64(&s.c.paused) != 0 {
		return driver.ErrXrun
	}
	return nil
}

//tymbal:rt
func (s *inputCapture) available() bool {
	return atomic.LoadUint64(&s.c.head)-atomic.LoadUint64(&s.c.tail) >= s.period
}

// read copies one complete input period before rendering or monitoring. It
// never substitutes silence for missing capture data.
//
//tymbal:rt
func (s *inputCapture) read(rate int) error {
	tail := atomic.LoadUint64(&s.c.tail)
	span := s.spans[s.metaTail&s.c.mask]
	position := math.Float64frombits(span.sampleTime) + float64(tail-span.start)
	next := tail + s.period
	known := span.flags&1 != 0
	if known && (math.IsNaN(position) || math.IsInf(position, 0)) {
		return errInputClock
	}
	if known && s.clockKnown && math.Abs(position-s.lastEnd) > 0.5 {
		return errInputClock
	}
	for index := s.metaTail; index < atomic.LoadUint64(&s.c.metaHead); index++ {
		item := s.spans[index&s.c.mask]
		if item.start >= next {
			break
		}
		if item.flags&1 == 0 {
			known = false
			continue
		}
		sampleTime := math.Float64frombits(item.sampleTime)
		if math.IsNaN(sampleTime) || math.IsInf(sampleTime, 0) || known && math.Abs(sampleTime-(position+float64(int64(item.start-tail)))) > 0.5 {
			return errInputClock
		}
	}
	index := tail & s.c.mask
	first := min(s.period, s.c.mask+1-index)
	fb := s.c.frameBytes
	split := int(first * fb)
	copy(s.staging[:split], s.data[int(index*fb):int((index+first)*fb)])
	copy(s.staging[split:], s.data[:len(s.staging)-split])
	// Timestamp spans are published with the PCM head and remain immutable
	// until this consumer releases the corresponding frames. No seqlock
	// retry or timestamp approximation is needed, even across IO boundaries.
	delta := int64(tail - span.start)
	s.pending = driver.ClockSample{}
	if known {
		frame := math.Float64frombits(span.sampleTime) + float64(delta)
		if frame >= 0 && frame < float64(math.MaxUint64) {
			s.pending.InputPosition = uint64(math.Round(frame))
			s.pending.InputFrequency = uint64(rate)
		}
	}
	if span.flags&2 != 0 && s.clock.denom != 0 {
		s.pending.InputQPCNano = s.clock.nano(span.hostTime) + delta*int64(time.Second)/int64(rate)
	}
	for s.metaTail < atomic.LoadUint64(&s.c.metaHead) && s.spans[s.metaTail&s.c.mask].end <= next {
		s.metaTail++
	}
	s.lastEnd, s.clockKnown = position+float64(s.period), known
	atomic.StoreUint64(&s.c.tail, tail+s.period)
	return nil
}

//tymbal:rt
func (s *inputCapture) recover() {
	atomic.StoreUint64(&s.c.tail, atomic.LoadUint64(&s.c.head))
	s.metaTail = atomic.LoadUint64(&s.c.metaHead)
	s.pending = driver.ClockSample{}
	s.clockKnown = false
	atomic.StoreUint64(&s.c.paused, 0)
}

//go:build darwin && (amd64 || arm64)

package coreaudio

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"m31labs.dev/tymbal/internal/coreaudio/testabi"
	"m31labs.dev/tymbal/internal/driver"
	"m31labs.dev/tymbal/internal/format"
	"m31labs.dev/tymbal/internal/ring"
	"m31labs.dev/tymbal/internal/rt"
)

type fakeInputDevice struct {
	status                  int32
	_                       uint32
	source                  uintptr
	frameBytes, frames, bus uint64
	memcpy                  uintptr
	flags                   uint64
}

type captureFixture struct {
	s      *stream
	device *fakeInputDevice
	source []byte
}

func newCaptureFixture(t *testing.T, channels, period, capacity, maxFrames int, duplex bool) *captureFixture {
	t.Helper()
	outputs := 0
	if duplex {
		outputs = 2
	}
	s, err := newStream(streamConfig{rate: 48000, channels: outputs, inputChannels: channels, period: period, fill: period * 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	input, err := newInputCapture(channels, period, capacity, maxFrames, s.sem)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.input.arena.Close(); err != nil {
		t.Fatal(err)
	}
	s.input = input
	s.input.clock = hostClock{numer: 1, denom: 1}
	s.c.sigmask, s.input.c.sigmask = 0, 0
	arena, err := ring.NewArena(128 + maxFrames*channels*4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = arena.Close() })
	control := arena.Take(64, 64)
	source := arena.Take(maxFrames*channels*4, 64)
	device := (*fakeInputDevice)(unsafe.Pointer(&control[0]))
	device.source, device.frameBytes, device.memcpy = uintptr(unsafe.Pointer(&source[0])), uint64(channels*4), fnmemcpy
	s.input.c.unit, s.input.c.render = uintptr(unsafe.Pointer(device)), testabi.InputRenderABI

	return &captureFixture{s: s, device: device, source: source}
}

func (f *captureFixture) fill(from uint64, frames int) {
	for k := 0; k < frames; k++ {
		for ch := 0; ch < int(f.device.frameBytes)/4; ch++ {
			binary.LittleEndian.PutUint32(f.source[k*int(f.device.frameBytes)+ch*4:], pattern(from+uint64(k), ch))
		}
	}
}

func inputStamp(frame uint64) [64]byte {
	var ts [64]byte
	binary.LittleEndian.PutUint64(ts[:], math.Float64bits(float64(1000+frame)))
	binary.LittleEndian.PutUint64(ts[8:], 1000000000+frame*uint64(time.Second)/48000)
	binary.LittleEndian.PutUint32(ts[56:], 3)
	return ts
}

func (f *captureFixture) push(frame uint64, frames int) {
	ts := inputStamp(frame)
	_, _, _ = syscall_syscall6(captureABI0, uintptr(unsafe.Pointer(f.s.input.c)), 0, uintptr(unsafe.Pointer(&ts[0])), 1, uintptr(frames), 0)
	runtime.KeepAlive(f)
}

func TestCaptureBlocksSignalsOnReplacementThreadAndAfterNativeFailure(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var before uint32
	if status := pthreadSigmask(1, nil, &before); status != 0 {
		t.Fatal(status)
	}
	defer pthreadSigmask(3, &before, nil)
	f := newCaptureFixture(t, 1, 4, 16, 16, false)
	f.s.input.c.sigmask = fnpthread_sigmask
	for _, failed := range []bool{false, false, true} {
		pthreadSigmask(3, &before, nil)
		if failed {
			atomic.StoreInt64(&f.s.input.c.status, -1)
		}
		f.push(0, 4)
		var got uint32
		if status := pthreadSigmask(1, nil, &got); status != 0 {
			t.Fatal(status)
		}
		if want := before | blockedSignals; got != want {
			t.Fatalf("foreign input thread mask=%#x, want %#x (failed=%v)", got, want, failed)
		}
	}
}

func TestCaptureForeignABIAndTimestampSpans(t *testing.T) {
	for _, channels := range []int{1, 2} {
		f := newCaptureFixture(t, channels, 4, 16, 16, false)
		if unsafe.Offsetof(f.device.source) != 8 || unsafe.Offsetof(f.device.flags) != 48 || unsafe.Sizeof(captureSpan{}) != 40 {
			t.Fatal("foreign fixture or timestamp layout changed")
		}
		var head, tail uint64
		for _, frames := range []int{3, 7, 1, 9, 4, 5, 7, 4} {
			f.fill(head, frames)
			f.push(head, frames)
			if err := f.s.input.waitError(); err != nil {
				t.Fatal(err)
			}
			if f.device.frames != uint64(frames) || f.device.bus != 1 {
				t.Fatalf("AudioUnitRender arguments lost: %+v", f.device)
			}
			head += uint64(frames)
			for f.s.input.available() {
				f.s.input.read(48000)
				expectFrames(t, f.s.input.staging, channels, tail, 4)
				tm := f.s.input.pending
				wantNano := int64(1000000000 + tail*uint64(time.Second)/48000)
				if tm.InputPosition != 1000+tail || tm.InputFrequency != 48000 || tm.InputQPCNano < wantNano-1 || tm.InputQPCNano > wantNano+1 {
					t.Fatalf("timestamp lost across IO boundaries: tail=%d got=%+v want_nano=%d", tail, tm, wantNano)
				}
				tail += 4
			}
		}
		if head != tail || f.s.input.c.overflows != 0 {
			t.Fatalf("capture lost frames: head=%d tail=%d", head, tail)
		}
	}
}

func TestCaptureOverflowRetainsCompletePeriodsAndRecoversWithNewClock(t *testing.T) {
	f := newCaptureFixture(t, 2, 4, 8, 16, false)
	f.fill(0, 8)
	f.push(0, 8)
	f.fill(8, 4)
	f.push(8, 4)
	if f.s.input.waitError() != driver.ErrXrun || f.s.input.c.head != 8 || f.s.input.c.droppedFrames != 4 {
		t.Fatal("overrun published overwritten or partial frames")
	}
	// Another callback remains paused, rather than joining newer data onto
	// old PCM with a fabricated contiguous timestamp.
	f.push(12, 4)
	if f.s.input.c.head != 8 || f.s.input.c.droppedFrames != 8 {
		t.Fatal("paused producer changed the capture queue")
	}
	f.s.input.recover()
	f.fill(16, 4)
	f.push(16, 4)
	if err := f.s.input.waitError(); err != nil {
		t.Fatal(err)
	}
	f.s.input.read(48000)
	expectFrames(t, f.s.input.staging, 2, 16, 4)
	if f.s.input.pending.InputPosition != 1016 || f.s.input.c.overflows != 1 {
		t.Fatal("recovery concealed the hardware clock gap")
	}
}

func TestCaptureSilenceAndErrorsDoNotReplayOldInput(t *testing.T) {
	f := newCaptureFixture(t, 1, 4, 16, 8, false)
	f.fill(0, 4)
	f.push(0, 4)
	f.s.input.read(48000)
	f.device.flags = 16 // AudioUnitRender declares this new block silent.
	f.push(4, 4)
	f.s.input.read(48000)
	for _, b := range f.s.input.staging {
		if b != 0 {
			t.Fatal("declared silence replayed previous input")
		}
	}
	f.device.status = -10847
	f.push(8, 4)
	if f.s.input.waitError() == nil || f.s.input.c.head != 8 {
		t.Fatal("failed native render produced capture data")
	}
	if n := testing.AllocsPerRun(100, func() { _ = f.s.input.waitError() }); n != 0 {
		t.Fatalf("native failure allocated %g objects", n)
	}
}

func TestDuplexRawInputAndOutputDoNotAllocate(t *testing.T) {
	f := newCaptureFixture(t, 2, 64, 256, 128, true)
	if err := f.s.Start(); err != nil {
		t.Fatal(err)
	}
	dst := make([]byte, 64*8)
	f.fill(0, 64)
	var frame uint64
	cycle := func() {
		f.push(frame, 64)
		frame += 64
		if callRender(f.s.c, dst, 64, 2) != 0 {
			panic("foreign render failed")
		}
		if err := f.s.Wait(); err != nil {
			panic(err)
		}
		in, out := f.s.Buffers()
		copy(out, in)
		if err := f.s.Commit(); err != nil {
			panic(err)
		}
	}
	if n := testing.AllocsPerRun(1000, cycle); n != 0 {
		t.Fatalf("duplex callback protocol allocated %g objects", n)
	}
	expectFrames(t, f.s.input.staging, 2, 0, 64)
	if f.s.Dropouts() != 0 {
		t.Fatalf("duplex stalled: %d dropouts", f.s.Dropouts())
	}
	if f.s.Params().InFormat != format.F32LE {
		t.Fatal("input format not negotiated")
	}
	if err := f.s.Stop(); err != nil {
		t.Fatal(err)
	}
	before := f.s.Dropouts()
	if err := f.s.Close(); err != nil {
		t.Fatal(err)
	}
	if f.s.Dropouts() != before {
		t.Fatal("close lost capture counters")
	}
}

func TestInputOnlyWaitCanBeInterruptedWithoutInput(t *testing.T) {
	f := newCaptureFixture(t, 1, 64, 256, 128, false)
	if err := f.s.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- f.s.Wait() }()
	f.s.Interrupt()
	select {
	case err := <-done:
		if err != driver.ErrInterrupted {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("input wait did not wake")
	}
	if atomic.LoadUint64(&f.s.input.c.head) != 0 {
		t.Fatal("interrupt fabricated input")
	}
}

func TestInputOnlyCapturesCompletePeriodsWithoutOutput(t *testing.T) {
	f := newCaptureFixture(t, 1, 64, 256, 128, false)
	if err := f.s.Start(); err != nil {
		t.Fatal(err)
	}
	f.fill(0, 64)
	f.push(0, 64)
	if err := f.s.Wait(); err != nil {
		t.Fatal(err)
	}
	in, out := f.s.Buffers()
	if len(out) != 0 {
		t.Fatal("input-only stream fabricated output")
	}
	expectFrames(t, in, 1, 0, 64)
	if output, input := f.s.Clock(); output != 0 || input != 1000000000 {
		t.Fatalf("input-only clock: out=%d in=%d", output, input)
	}
	if err := f.s.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentCapturePreservesRawFramesAndClockAcrossIrregularIO(t *testing.T) {
	const periods = 1000
	f := newCaptureFixture(t, 2, 64, 512, 128, false)
	if err := f.s.Start(); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	t.Cleanup(func() { close(stop); f.s.Interrupt(); <-done })
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		var head uint64
		chunks := []int{17, 31, 64, 97, 3}
		for cycle := 0; head < periods*64; cycle++ {
			frames := min(chunks[cycle%len(chunks)], periods*64-int(head))
			for uint64(frames) > f.s.input.c.mask+1-(head-atomic.LoadUint64(&f.s.input.c.tail)) {
				select {
				case <-stop:
					return
				default:
					runtime.Gosched()
				}
			}
			select {
			case <-stop:
				return
			default:
			}
			f.fill(head, frames)
			f.push(head, frames)
			head += uint64(frames)
		}
	}()
	for period := 0; period < periods; period++ {
		if err := f.s.Wait(); err != nil {
			t.Fatal(err)
		}
		in, out := f.s.Buffers()
		if len(out) != 0 {
			t.Fatal("input-only producer emitted output")
		}
		expectFrames(t, in, 2, uint64(period*64), 64)
		clock := f.s.ClockSample()
		if clock.InputPosition != uint64(1000+period*64) || clock.InputFrequency != 48000 {
			t.Fatalf("period %d clock: %+v", period, clock)
		}
		if err := f.s.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	<-done
	if f.s.Dropouts() != 0 {
		t.Fatalf("concurrent capture lost frames: %d", f.s.Dropouts())
	}
}

func TestCaptureClockGapCannotBeHiddenInsideOnePeriod(t *testing.T) {
	for _, gap := range []uint64{0, 20} {
		f := newCaptureFixture(t, 1, 4, 16, 8, false)
		f.fill(0, 2)
		f.push(0, 2)
		f.fill(gap, 2)
		f.push(gap, 2)
		if err := f.s.input.read(48000); !errors.Is(err, driver.ErrLost) {
			t.Fatalf("mixed timestamps produced a fake contiguous period: %v", err)
		}
		if f.s.input.c.tail != 0 {
			t.Fatal("invalid period was consumed")
		}
	}
}

func TestCaptureRejectsClockRegressionBetweenPeriods(t *testing.T) {
	f := newCaptureFixture(t, 1, 4, 16, 8, false)
	f.fill(0, 4)
	f.push(0, 4)
	if err := f.s.input.read(48000); err != nil {
		t.Fatal(err)
	}
	f.push(0, 4)
	if err := f.s.input.read(48000); !errors.Is(err, driver.ErrLost) {
		t.Fatalf("device clock restart was concealed: %v", err)
	}
}

func TestHardwareClockSharesTheStreamMonotonicDomain(t *testing.T) {
	clock := newHostClock()
	if clock.denom == 0 {
		t.Fatal("Mach timebase unavailable")
	}
	before := rt.Now()
	native := clock.nano(captureMachTime())
	after := rt.Now()
	if native < before-int64(time.Millisecond) || native > after+int64(time.Millisecond) {
		t.Fatalf("clock domains differ: before=%d native=%d after=%d", before, native, after)
	}
}

// This metadata-only gate never initializes or starts an input audio unit.
// It verifies private clock composition without acquiring microphone samples.
func TestPrivateDuplexDeviceDoesNotChangeDefaultsOrLeakIntoInventory(t *testing.T) {
	if os.Getenv("TYMBAL_AGGREGATE_TEST") != "1" {
		t.Skip("set TYMBAL_AGGREGATE_TEST=1 for the native device-composition gate")
	}
	b := New()
	out, err := b.Default(dirOutput)
	if err != nil {
		t.Fatal(err)
	}
	in, err := b.Default(dirInput)
	if err != nil {
		t.Fatal(err)
	}
	if in.ID == out.ID {
		t.Skip("default endpoints already share a device clock")
	}
	device, err := newDuplexDevice(out.ID, in.ID, out.Outputs, in.Inputs+out.Inputs)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := check("destroy aggregate", destroyAggregateDevice(device)); err != nil {
			t.Error(err)
		}
	}()
	if channelCount(device, scopeInput) < in.Inputs || channelCount(device, scopeOutput) < out.Outputs {
		t.Fatal("private clock lost channels")
	}
	devices, err := b.Devices()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range devices {
		if item.Name == "Tymbal duplex clock" {
			t.Fatal("internal clock appeared as a selectable device")
		}
	}
	currentOut, err := b.Default(dirOutput)
	if err != nil || currentOut.ID != out.ID {
		t.Fatal("composition changed the default output")
	}
	currentIn, err := b.Default(dirInput)
	if err != nil || currentIn.ID != in.ID {
		t.Fatal("composition changed the default input")
	}
}

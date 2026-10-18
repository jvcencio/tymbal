//go:build darwin

package rt

import (
	"testing"
	"time"
)

func TestSleepThreadDoesNotAllocate(t *testing.T) {
	if allocations := testing.AllocsPerRun(100, func() { SleepThread(time.Microsecond) }); allocations != 0 {
		t.Fatalf("native thread sleep allocated %g objects", allocations)
	}
	SleepThread(0)
	SleepThread(-time.Second)
}

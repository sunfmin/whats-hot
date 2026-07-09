package dashboard

import (
	"testing"

	"github.com/sunfmin/whats-hot/internal/monitor"
)

// snap builds a one-process, one-Established-flow snapshot at the given rates.
func snap(down, up int64) monitor.Snapshot {
	return monitor.Snapshot{
		Processes: []monitor.Process{{
			Name: "curl", PID: 999, DownBps: down, UpBps: up,
			Flows: []monitor.Flow{{
				Proto: "tcp4", State: "Established",
				Local:  monitor.Endpoint{IP: "10.0.0.2", Port: 5000},
				Remote: monitor.Endpoint{IP: "1.2.3.4", Port: 443, Host: "cdn.example.com"},
				DownBps: down, UpBps: up,
			}},
		}},
	}
}

func TestStallAlertsOnceAfterWindow(t *testing.T) {
	tr := newStallTracker(1000, 3)

	if a := tr.check(snap(50000, 0)); len(a) != 0 { // moving: no alert
		t.Fatalf("moving flow alerted: %v", a)
	}
	if a := tr.check(snap(0, 0)); len(a) != 0 { // zero #1
		t.Fatalf("premature alert at zero#1: %v", a)
	}
	if a := tr.check(snap(0, 0)); len(a) != 0 { // zero #2
		t.Fatalf("premature alert at zero#2: %v", a)
	}
	a := tr.check(snap(0, 0)) // zero #3 == window → alert
	if len(a) != 1 {
		t.Fatalf("expected 1 alert at window, got %d", len(a))
	}
	if a2 := tr.check(snap(0, 0)); len(a2) != 0 { // stays stalled: no repeat
		t.Fatalf("alert repeated: %v", a2)
	}
	// recovers, then stalls again → alerts again
	tr.check(snap(50000, 0))
	tr.check(snap(0, 0))
	tr.check(snap(0, 0))
	if a := tr.check(snap(0, 0)); len(a) != 1 {
		t.Fatalf("expected re-alert after recovery, got %d", len(a))
	}
}

func TestNeverMovingDoesNotAlert(t *testing.T) {
	tr := newStallTracker(1000, 2)
	for i := 0; i < 5; i++ {
		if a := tr.check(snap(0, 0)); len(a) != 0 {
			t.Fatalf("idle-from-start flow alerted at step %d", i)
		}
	}
}

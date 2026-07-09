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
				Local:   monitor.Endpoint{IP: "10.0.0.2", Port: 5000},
				Remote:  monitor.Endpoint{IP: "1.2.3.4", Port: 443, Host: "cdn.example.com"},
				DownBps: down, UpBps: up,
			}},
		}},
	}
}

// testCfg is small and cooldown-free so window/sustain behaviour is easy to assert.
func testCfg(sustain, window int) stallConfig {
	return stallConfig{moveBps: 1000, stallBps: 100, sustain: sustain, window: window, cooldown: 0}
}

func TestStallAlertsOnceAfterSustainedMoveThenWindow(t *testing.T) {
	tr := newStallTracker(testCfg(2, 3))

	tr.check(snap(50000, 0)) // move #1 (not yet eligible: sustain=2)
	if a := tr.check(snap(50000, 0)); len(a) != 0 {
		t.Fatalf("moving flow alerted: %v", a) // move #2 → eligible, still no alert
	}
	if a := tr.check(snap(0, 0)); len(a) != 0 { // zero #1
		t.Fatalf("premature alert at zero#1: %v", a)
	}
	if a := tr.check(snap(0, 0)); len(a) != 0 { // zero #2
		t.Fatalf("premature alert at zero#2: %v", a)
	}
	if a := tr.check(snap(0, 0)); len(a) != 1 { // zero #3 == window → alert
		t.Fatalf("expected 1 alert at window, got %d", len(a))
	}
	if a := tr.check(snap(0, 0)); len(a) != 0 { // stays stalled: no repeat
		t.Fatalf("alert repeated: %v", a)
	}
	// recovers (sustained again), then stalls again → alerts again
	tr.check(snap(50000, 0))
	tr.check(snap(50000, 0))
	tr.check(snap(0, 0))
	tr.check(snap(0, 0))
	if a := tr.check(snap(0, 0)); len(a) != 1 {
		t.Fatalf("expected re-alert after recovery, got %d", len(a))
	}
}

func TestBriefBlipDoesNotAlert(t *testing.T) {
	tr := newStallTracker(testCfg(3, 2)) // needs 3 moving snapshots to be eligible
	tr.check(snap(50000, 0))             // a single blip — never sustained
	for i := 0; i < 6; i++ {
		if a := tr.check(snap(0, 0)); len(a) != 0 {
			t.Fatalf("brief-blip flow alerted at step %d: %v", i, a)
		}
	}
}

func TestTrickleIsNotAStall(t *testing.T) {
	tr := newStallTracker(testCfg(1, 2))
	tr.check(snap(50000, 0)) // eligible
	for i := 0; i < 6; i++ { // trickling above stallBps but below moveBps → never stalled
		if a := tr.check(snap(200, 0)); len(a) != 0 {
			t.Fatalf("trickling flow alerted at step %d: %v", i, a)
		}
	}
}

func TestNeverMovingDoesNotAlert(t *testing.T) {
	tr := newStallTracker(testCfg(1, 2))
	for i := 0; i < 5; i++ {
		if a := tr.check(snap(0, 0)); len(a) != 0 {
			t.Fatalf("idle-from-start flow alerted at step %d", i)
		}
	}
}

func TestCooldownRateLimitsBanners(t *testing.T) {
	cfg := stallConfig{moveBps: 1000, stallBps: 100, sustain: 1, window: 1, cooldown: 100}
	tr := newStallTracker(cfg)

	tr.check(snap(50000, 0))                    // eligible (sustain=1)
	if a := tr.check(snap(0, 0)); len(a) != 1 { // window=1 → first banner fires
		t.Fatalf("expected first banner, got %d", len(a))
	}
	tr.check(snap(50000, 0))                    // recovers
	if a := tr.check(snap(0, 0)); len(a) != 0 { // stalls again within cooldown → suppressed
		t.Fatalf("cooldown did not suppress repeat banner: %v", a)
	}
}

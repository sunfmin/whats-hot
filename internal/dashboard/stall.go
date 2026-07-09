package dashboard

import (
	"fmt"

	"github.com/sunfmin/whats-hot/internal/monitor"
)

type alertMsg struct{ title, msg string }

// stallTracker watches Established flows across successive snapshots and flags an
// Observed Transfer as stalled once it has been moving and then sits below `threshold`
// bytes/sec for `window` consecutive snapshots. Each stall alerts at most once until the
// flow moves again (see docs/CONTEXT.md: Stall, Alert).
type stallTracker struct {
	threshold int64
	window    int
	active    map[string]bool
	zero      map[string]int
	alerted   map[string]bool
}

func newStallTracker(threshold int64, window int) *stallTracker {
	return &stallTracker{
		threshold: threshold, window: window,
		active:  map[string]bool{},
		zero:    map[string]int{},
		alerted: map[string]bool{},
	}
}

func (t *stallTracker) check(snap monitor.Snapshot) []alertMsg {
	var alerts []alertMsg
	seen := map[string]bool{}
	for _, p := range snap.Processes {
		for _, f := range p.Flows {
			if f.State != "Established" {
				continue
			}
			key := fmt.Sprintf("%d/%s %s:%d->%s:%d", p.PID, f.Proto,
				f.Local.IP, f.Local.Port, f.Remote.IP, f.Remote.Port)
			seen[key] = true
			if f.DownBps+f.UpBps >= t.threshold {
				t.active[key] = true
				t.zero[key] = 0
				t.alerted[key] = false
				continue
			}
			if !t.active[key] { // never moving → idle, not a stall
				continue
			}
			t.zero[key]++
			if t.zero[key] >= t.window && !t.alerted[key] {
				t.alerted[key] = true
				host := f.Remote.Host
				if host == "" {
					host = f.Remote.IP
				}
				alerts = append(alerts, alertMsg{
					title: "Transfer stalled",
					msg: fmt.Sprintf("%s ↔ %s looks stuck (~0 B/s for %ds). Retry it in that app.",
						p.Name, host, t.zero[key]),
				})
			}
		}
	}
	for k := range t.active { // forget closed flows
		if !seen[k] {
			delete(t.active, k)
			delete(t.zero, k)
			delete(t.alerted, k)
		}
	}
	return alerts
}

package dashboard

import (
	"fmt"

	"github.com/sunfmin/whats-hot/internal/monitor"
)

type alertMsg struct{ title, msg string }

// stallConfig tunes when an Observed Transfer is considered stuck. The defaults are
// deliberately conservative: a flow must carry a real, sustained transfer and then fully
// stop before it alerts, and banners are globally rate-limited — otherwise a proxy's
// normal churn of short-lived flows (blip → quiet) would spam Notification Center.
type stallConfig struct {
	moveBps  int64 // >= this bytes/sec counts as an active transfer (eligibility bar)
	stallBps int64 // <= this bytes/sec counts as stopped (a mere trickle is neither)
	sustain  int   // consecutive moving snapshots before a flow can ever be flagged
	window   int   // consecutive stopped snapshots before flagging a stall
	cooldown int   // minimum snapshots between any two emitted banners (global limit)
}

func defaultStallConfig() stallConfig {
	return stallConfig{moveBps: 64 * 1024, stallBps: 4 * 1024, sustain: 5, window: 15, cooldown: 120}
}

// stallTracker watches Established flows across successive snapshots and flags an
// Observed Transfer as stalled once it has carried a sustained transfer and then sits
// stopped for `window` consecutive snapshots. Each stall alerts at most once until the
// flow moves again, and a global cooldown caps how often any banner fires (see
// docs/CONTEXT.md: Stall, Alert).
type stallTracker struct {
	cfg stallConfig

	moving  map[string]int  // consecutive moving snapshots (builds toward eligibility)
	active  map[string]bool // has carried a sustained transfer → eligible to stall
	zero    map[string]int  // consecutive stopped snapshots
	alerted map[string]bool // already banered for the current stall

	tick      int
	lastAlert int // tick of the last emitted banner (for the global cooldown)
}

func newStallTracker(cfg stallConfig) *stallTracker {
	return &stallTracker{
		cfg:       cfg,
		moving:    map[string]int{},
		active:    map[string]bool{},
		zero:      map[string]int{},
		alerted:   map[string]bool{},
		lastAlert: -cfg.cooldown, // let the first genuine stall alert immediately
	}
}

func (t *stallTracker) check(snap monitor.Snapshot) []alertMsg {
	t.tick++
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
			bps := f.DownBps + f.UpBps

			if bps >= t.cfg.moveBps { // real transfer: (re)build eligibility, clear any stall
				t.moving[key]++
				t.zero[key] = 0
				t.alerted[key] = false
				if t.moving[key] >= t.cfg.sustain {
					t.active[key] = true
				}
				continue
			}
			t.moving[key] = 0
			if bps > t.cfg.stallBps { // trickling, not stopped — not a stall
				t.zero[key] = 0
				continue
			}
			if !t.active[key] { // never carried a sustained transfer → idle, not a stall
				continue
			}
			t.zero[key]++
			if t.zero[key] >= t.cfg.window && !t.alerted[key] {
				t.alerted[key] = true // one banner per stall, even if the cooldown drops it
				if t.tick-t.lastAlert < t.cfg.cooldown {
					continue // global rate-limit: skip the banner but stay flagged
				}
				t.lastAlert = t.tick
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
			delete(t.moving, k)
			delete(t.zero, k)
			delete(t.alerted, k)
		}
	}
	return alerts
}

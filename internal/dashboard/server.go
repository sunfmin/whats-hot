// Package dashboard is the human-facing surface: a local HTTP server that renders the
// activity page with htmlgo and streams live Snapshots to it over Server-Sent Events.
package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/sunfmin/whats-hot/internal/alert"
	"github.com/sunfmin/whats-hot/internal/monitor"
)

// Server broadcasts monitor Snapshots to connected browsers via SSE and raises Alerts
// when an Observed Transfer stalls.
type Server struct {
	resolver *monitor.Resolver
	stall    *stallTracker
	alertFn  func(title, msg string)

	mu      sync.Mutex
	latest  monitor.Snapshot
	haveOne bool
	clients map[chan monitor.Snapshot]struct{}
}

func New() *Server {
	return &Server{
		resolver: monitor.NewResolver(),
		stall:    newStallTracker(1024, 10),
		alertFn:  func(t, m string) { _ = alert.Notify(t, m) },
		clients:  map[chan monitor.Snapshot]struct{}{},
	}
}

// Serve streams nettop into the SSE fan-out and serves the dashboard on the given
// listener until ctx is cancelled. The caller owns the listener (so it can report the
// bound address before serving).
func (s *Server) Serve(ctx context.Context, l net.Listener) error {
	snaps := make(chan monitor.Snapshot, 4)
	go func() {
		// Stream ends when nettop exits or ctx is cancelled; either way close the fan-in.
		_ = monitor.Stream(ctx, s.resolver, snaps)
		close(snaps)
	}()
	go s.pump(snaps)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Render(w)
	})
	mux.HandleFunc("/events", s.events)

	srv := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	if err := srv.Serve(l); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// pump consumes snapshots, updates the latest cache, runs stall detection, and fans out.
func (s *Server) pump(snaps <-chan monitor.Snapshot) {
	for snap := range snaps {
		for _, a := range s.stall.check(snap) {
			s.alertFn(a.title, a.msg)
		}
		s.mu.Lock()
		s.latest = snap
		s.haveOne = true
		for ch := range s.clients {
			select {
			case ch <- snap:
			default: // slow client: drop this tick rather than block the pump
			}
		}
		s.mu.Unlock()
	}
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan monitor.Snapshot, 4)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	latest, haveOne := s.latest, s.haveOne
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, ch)
		s.mu.Unlock()
	}()

	if haveOne {
		writeEvent(w, flusher, latest)
	}
	for {
		select {
		case snap := <-ch:
			writeEvent(w, flusher, snap)
		case <-r.Context().Done():
			return
		}
	}
}

func writeEvent(w http.ResponseWriter, flusher http.Flusher, snap monitor.Snapshot) {
	b, err := json.Marshal(snap)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", b)
	flusher.Flush()
}

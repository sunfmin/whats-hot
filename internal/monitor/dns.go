package monitor

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"
)

// Resolver turns remote IPs into hostnames via reverse DNS, caching results (including
// misses) so a live dashboard resolves each IP at most once. It is safe for concurrent
// use. This is the no-sudo path to a Remote Endpoint's Host; --deep SNI is layered on
// top elsewhere.
type Resolver struct {
	cache   sync.Map // ip string -> host string ("" = looked up, no name)
	timeout time.Duration
}

func NewResolver() *Resolver { return &Resolver{timeout: 400 * time.Millisecond} }

// Host returns the reverse-DNS name for ip, or "" if none/unresolvable. Wildcards and
// local addresses are skipped without a lookup.
func (r *Resolver) Host(ip string) string {
	if skipReverse(ip) {
		return ""
	}
	if v, ok := r.cache.Load(ip); ok {
		return v.(string)
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	host := ""
	if names, err := net.DefaultResolver.LookupAddr(ctx, ip); err == nil && len(names) > 0 {
		host = strings.TrimSuffix(names[0], ".")
	}
	r.cache.Store(ip, host)
	return host
}

// Annotate fills Remote.Host for every flow in the snapshot.
func (r *Resolver) Annotate(snap *Snapshot) {
	for pi := range snap.Processes {
		for fi := range snap.Processes[pi].Flows {
			fl := &snap.Processes[pi].Flows[fi]
			if fl.Remote.Host == "" {
				fl.Remote.Host = r.Host(fl.Remote.IP)
			}
		}
	}
}

func skipReverse(ip string) bool {
	if ip == "" || ip == "*" {
		return true
	}
	if i := strings.IndexByte(ip, '%'); i >= 0 { // strip IPv6 zone
		ip = ip[:i]
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return true
	}
	return parsed.IsLoopback() || parsed.IsLinkLocalUnicast() || parsed.IsUnspecified()
}

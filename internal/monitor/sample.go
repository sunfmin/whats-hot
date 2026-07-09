package monitor

import (
	"bufio"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A frame is one nettop sample: processes keyed by "name.pid", each with its flows.
type frame struct {
	order []string            // process keys in first-seen order
	procs map[string]*Process // key -> process (Flows attached during parse)
}

func newFrame() *frame { return &frame{procs: map[string]*Process{}} }

const nettopHeaderPrefix = "time,,interface,"

// clock lets tests inject a deterministic timestamp.
type clock interface{ now() time.Time }

type realClock struct{}

func (realClock) now() time.Time { return time.Now() }

type fixedClock struct{ t time.Time }

func (c fixedClock) now() time.Time { return c.t }

// frameScanner yields nettop sample frames one at a time from a stream. A frame
// starts at a repeated CSV header line and ends at the next header (or EOF), which
// makes it usable both for a finished capture and for a live nettop pipe.
type frameScanner struct {
	sc      *bufio.Scanner
	pending *frame
	curProc *Process
}

func newFrameScanner(r io.Reader) *frameScanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return &frameScanner{sc: sc}
}

// next blocks until the next complete frame is available, returning ok=false at EOF.
func (fs *frameScanner) next() (*frame, bool) {
	for fs.sc.Scan() {
		line := fs.sc.Text()
		if strings.HasPrefix(line, nettopHeaderPrefix) {
			if fs.pending != nil && len(fs.pending.order) > 0 {
				done := fs.pending
				fs.pending = newFrame()
				fs.curProc = nil
				return done, true
			}
			fs.pending = newFrame()
			fs.curProc = nil
			continue
		}
		if fs.pending == nil || line == "" {
			continue
		}
		fs.applyRow(line)
	}
	if fs.pending != nil && len(fs.pending.order) > 0 {
		done := fs.pending
		fs.pending = nil
		return done, true
	}
	return nil, false
}

func (fs *frameScanner) applyRow(line string) {
	f := strings.Split(line, ",")
	if len(f) < 6 {
		return
	}
	name := f[1]
	if strings.Contains(name, "<->") {
		if fs.curProc == nil {
			return
		}
		if fl := parseFlow(f); fl != nil {
			fs.curProc.Flows = append(fs.curProc.Flows, *fl)
		}
		return
	}
	procName, pid, ok := splitNamePID(name)
	if !ok {
		return
	}
	p := &Process{Name: procName, PID: pid, BytesIn: atoi64(f[4]), BytesOut: atoi64(f[5])}
	fs.pending.procs[name] = p
	fs.pending.order = append(fs.pending.order, name)
	fs.curProc = p
}

func parseFlow(f []string) *Flow {
	head := f[1] // "tcp4 198.18.0.1:61672<->198.18.0.151:5223"
	sp := strings.IndexByte(head, ' ')
	if sp < 0 {
		return nil
	}
	proto, addrs := head[:sp], head[sp+1:]
	i := strings.Index(addrs, "<->")
	if i < 0 {
		return nil
	}
	return &Flow{
		Proto:     proto,
		Local:     parseEndpoint(proto, addrs[:i]),
		Remote:    parseEndpoint(proto, addrs[i+3:]),
		Interface: f[2],
		State:     f[3],
		BytesIn:   atoi64(f[4]),
		BytesOut:  atoi64(f[5]),
		RTTms:     parseLeadingFloat(f[9]),
	}
}

// parseEndpoint splits an address into IP and port. IPv4 (proto ending "4") uses ':'
// before the port; IPv6 (proto ending "6") uses '.' after the scoped address. A "*"
// port parses to 0.
func parseEndpoint(proto, s string) Endpoint {
	sep := byte(':')
	if strings.HasSuffix(proto, "6") {
		sep = '.'
	}
	ip, port := s, 0
	if i := strings.LastIndexByte(s, sep); i >= 0 {
		ip = s[:i]
		port = atoi(s[i+1:])
	}
	return Endpoint{IP: ip, Port: port}
}

func splitNamePID(s string) (name string, pid int, ok bool) {
	i := strings.LastIndexByte(s, '.')
	if i < 0 {
		return "", 0, false
	}
	pid, err := strconv.Atoi(s[i+1:])
	if err != nil {
		return "", 0, false
	}
	return s[:i], pid, true
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

// parseLeadingFloat reads the number at the start of "1.22 ms" -> 1.22.
func parseLeadingFloat(s string) float64 {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && (s[end] == '.' || s[end] == '-' || (s[end] >= '0' && s[end] <= '9')) {
		end++
	}
	v, _ := strconv.ParseFloat(s[:end], 64)
	return v
}

// diff computes throughput by subtracting cumulative counters between two frames
// captured interval seconds apart. Rates clamp at zero (a negative delta means a
// counter reset or a brand-new flow). The Snapshot carries the later frame's absolute
// counters plus the diffed rates, sorted by busiest process first.
func diff(prev, cur *frame, interval float64, ck clock) Snapshot {
	if interval <= 0 {
		interval = 1
	}
	rate := func(now, before int64) int64 {
		if d := now - before; d > 0 {
			return int64(float64(d) / interval)
		}
		return 0
	}
	snap := Snapshot{Time: ck.now(), IntervalSec: interval}
	for _, key := range cur.order {
		p := cur.procs[key]
		var prevFlows map[string]Flow
		if prev != nil {
			if old, ok := prev.procs[key]; ok {
				prevFlows = indexFlows(old.Flows)
			}
		}
		out := Process{
			Name: p.Name, PID: p.PID,
			BytesIn: p.BytesIn, BytesOut: p.BytesOut,
		}
		for _, fl := range p.Flows {
			var bin, bout int64
			if pf, ok := prevFlows[flowKey(fl)]; ok {
				bin, bout = pf.BytesIn, pf.BytesOut
			}
			fl.DownBps = rate(fl.BytesIn, bin)
			fl.UpBps = rate(fl.BytesOut, bout)
			// A process's throughput is the sum of its flow rates. nettop's own
			// process-level counter under-reports (often to zero) loopback and proxied
			// traffic, while the per-flow counters stay reliable — so for a proxied app
			// its lo0 flow to the proxy is what reveals its true download rate.
			out.DownBps += fl.DownBps
			out.UpBps += fl.UpBps
			// The machine total counts only real interfaces. Loopback (lo0) is local IPC
			// that nettop mirrors on both endpoints (down==up), so counting it would
			// double-count every proxied byte and dwarf the real link rate.
			if !isLoopback(fl.Interface) {
				snap.TotalDownBps += fl.DownBps
				snap.TotalUpBps += fl.UpBps
			}
			out.Flows = append(out.Flows, fl)
		}
		snap.Processes = append(snap.Processes, out)
	}
	sort.SliceStable(snap.Processes, func(i, j int) bool {
		return snap.Processes[i].DownBps+snap.Processes[i].UpBps >
			snap.Processes[j].DownBps+snap.Processes[j].UpBps
	})
	return snap
}

// isLoopback reports whether a flow rides a loopback device (lo0, lo1, …) — local IPC,
// not internet traffic, so it is excluded from the machine-wide total.
func isLoopback(iface string) bool {
	return strings.HasPrefix(iface, "lo")
}

func flowKey(f Flow) string {
	return f.Proto + " " + f.Local.IP + ":" + strconv.Itoa(f.Local.Port) +
		"<->" + f.Remote.IP + ":" + strconv.Itoa(f.Remote.Port)
}

func indexFlows(fs []Flow) map[string]Flow {
	m := make(map[string]Flow, len(fs))
	for _, f := range fs {
		m[flowKey(f)] = f
	}
	return m
}

// snapshotFromReader parses a full nettop capture and diffs the first and last frames,
// producing an average-throughput Snapshot over the whole window. Used by the snapshot
// command and by tests against captured fixtures.
func snapshotFromReader(r io.Reader, interval float64, ck clock) (Snapshot, bool) {
	fsr := newFrameScanner(r)
	var first, last *frame
	for {
		f, ok := fsr.next()
		if !ok {
			break
		}
		if first == nil {
			first = f
		}
		last = f
	}
	if last == nil {
		return Snapshot{}, false
	}
	if first == last {
		return diff(nil, last, interval, ck), true
	}
	return diff(first, last, interval, ck), true
}

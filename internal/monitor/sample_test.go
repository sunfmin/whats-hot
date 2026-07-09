package monitor

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		proto, in string
		wantIP    string
		wantPort  int
	}{
		{"tcp4", "198.18.0.151:5223", "198.18.0.151", 5223},
		{"tcp4", "*:*", "*", 0},
		{"tcp6", "fe80::7c7a:e0d6:b0fa:6231%utun5.1024", "fe80::7c7a:e0d6:b0fa:6231%utun5", 1024},
		{"tcp6", "*.*", "*", 0},
	}
	for _, tt := range tests {
		got := parseEndpoint(tt.proto, tt.in)
		if got.IP != tt.wantIP || got.Port != tt.wantPort {
			t.Errorf("parseEndpoint(%q,%q) = %s:%d, want %s:%d",
				tt.proto, tt.in, got.IP, got.Port, tt.wantIP, tt.wantPort)
		}
	}
}

func TestParseRealFixture(t *testing.T) {
	f, err := os.Open("testdata/nettop-tcp-2frames.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	fsr := newFrameScanner(f)
	var frames []*frame
	for {
		fr, ok := fsr.next()
		if !ok {
			break
		}
		frames = append(frames, fr)
	}
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want 2", len(frames))
	}

	// apsd.377 process row with its known cumulative counters.
	apsd := frames[0].procs["apsd.377"]
	if apsd == nil {
		t.Fatal("apsd.377 not parsed in frame 0")
	}
	if apsd.PID != 377 || apsd.Name != "apsd" {
		t.Errorf("apsd parsed as name=%q pid=%d", apsd.Name, apsd.PID)
	}
	if apsd.BytesIn != 14263 || apsd.BytesOut != 45582 {
		t.Errorf("apsd bytes = %d/%d, want 14263/45582", apsd.BytesIn, apsd.BytesOut)
	}
	// its Established flow to 198.18.0.151:5223 over utun4.
	var found *Flow
	for i := range apsd.Flows {
		if apsd.Flows[i].Remote.IP == "198.18.0.151" {
			found = &apsd.Flows[i]
		}
	}
	if found == nil {
		t.Fatal("apsd flow to 198.18.0.151 not found")
	}
	if found.Remote.Port != 5223 || found.Interface != "utun4" || found.State != "Established" {
		t.Errorf("flow parsed as %s:%d iface=%q state=%q",
			found.Remote.IP, found.Remote.Port, found.Interface, found.State)
	}
}

// row builds a nettop CSV row with enough columns to reach the rtt field (index 9).
func row(field1, iface, state string, in, out int64, rtt string) string {
	cols := make([]string, 20)
	cols[0] = "t"
	cols[1] = field1
	cols[2] = iface
	cols[3] = state
	cols[4] = strconv.FormatInt(in, 10)
	cols[5] = strconv.FormatInt(out, 10)
	cols[9] = rtt
	return strings.Join(cols, ",")
}

func TestDiffThroughput(t *testing.T) {
	header := "time,,interface,state,bytes_in,bytes_out,rx_dupe,rx_ooo,re-tx,rtt_avg,rest"
	flow := "tcp4 10.0.0.2:5000<->1.2.3.4:443"
	var b strings.Builder
	b.WriteString(header + "\n")
	b.WriteString(row("launchd.1", "", "", 0, 0, "") + "\n")
	b.WriteString(row("curl.999", "", "", 1000, 200, "") + "\n")
	b.WriteString(row(flow, "en0", "Established", 1000, 200, "1.0 ms") + "\n")
	b.WriteString(header + "\n")
	b.WriteString(row("launchd.1", "", "", 0, 0, "") + "\n")
	b.WriteString(row("curl.999", "", "", 6000, 700, "") + "\n")
	b.WriteString(row(flow, "en0", "Established", 6000, 700, "1.0 ms") + "\n")

	snap, ok := snapshotFromReader(strings.NewReader(b.String()), 1, fixedClock{time.Unix(0, 0)})
	if !ok {
		t.Fatal("snapshotFromReader returned ok=false")
	}
	if snap.TotalDownBps != 5000 || snap.TotalUpBps != 500 {
		t.Errorf("totals = %d down / %d up, want 5000/500", snap.TotalDownBps, snap.TotalUpBps)
	}
	// busiest process is curl, sorted first, with its flow rate diffed.
	if len(snap.Processes) == 0 || snap.Processes[0].Name != "curl" {
		t.Fatalf("expected curl first, got %+v", snap.Processes)
	}
	c := snap.Processes[0]
	if c.DownBps != 5000 || c.UpBps != 500 {
		t.Errorf("curl proc rate = %d/%d, want 5000/500", c.DownBps, c.UpBps)
	}
	if len(c.Flows) != 1 || c.Flows[0].DownBps != 5000 || c.Flows[0].UpBps != 500 {
		t.Errorf("curl flow rate = %+v, want 5000/500", c.Flows)
	}
}

func TestDiffClampsNegative(t *testing.T) {
	header := "time,,interface,state,bytes_in,bytes_out,rx_dupe,rx_ooo,re-tx,rtt_avg,rest"
	var b strings.Builder
	b.WriteString(header + "\n")
	b.WriteString(row("app.1", "", "", 9000, 9000, "") + "\n")
	b.WriteString(header + "\n")
	b.WriteString(row("app.1", "", "", 100, 100, "") + "\n") // counter went backwards
	snap, ok := snapshotFromReader(strings.NewReader(b.String()), 1, fixedClock{time.Unix(0, 0)})
	if !ok {
		t.Fatal("ok=false")
	}
	if snap.TotalDownBps != 0 || snap.TotalUpBps != 0 {
		t.Errorf("negative delta not clamped: %d/%d", snap.TotalDownBps, snap.TotalUpBps)
	}
}

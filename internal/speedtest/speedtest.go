// Package speedtest measures Link Capacity — the connection's benchmarked ceiling — by
// wrapping macOS's built-in networkQuality tool. This is distinct from Throughput (what
// is flowing right now); see docs/CONTEXT.md.
package speedtest

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// Result is the parsed networkQuality summary. Throughputs are bits/sec as reported by
// the tool; Responsiveness is in RPM (round-trips per minute).
type Result struct {
	DownlinkBps       float64        `json:"dl_throughput"`
	UplinkBps         float64        `json:"ul_throughput"`
	Responsiveness    float64        `json:"responsiveness"`
	BaseRTTms         float64        `json:"base_rtt"`
	InterfaceName     string         `json:"interface_name"`
	Raw               map[string]any `json:"-"`
}

// Run executes `networkQuality -c` (computer-readable JSON) and parses the result. The
// test typically takes 10-20s; pass a context with a generous deadline.
func Run(ctx context.Context) (*Result, error) {
	out, err := exec.CommandContext(ctx, "networkQuality", "-c").Output()
	if err != nil {
		return nil, fmt.Errorf("networkQuality: %w", err)
	}
	var r Result
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, fmt.Errorf("parse networkQuality output: %w", err)
	}
	_ = json.Unmarshal(out, &r.Raw) // keep everything for fields we don't model
	return &r, nil
}

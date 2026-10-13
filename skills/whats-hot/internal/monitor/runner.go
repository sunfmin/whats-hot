package monitor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
)

// nettopArgs builds the tcp-mode, parseable, 1-second-interval invocation. samples<=0
// means run continuously (-L 0 = infinite).
func nettopArgs(samples int) []string {
	l := "0"
	if samples > 0 {
		l = strconv.Itoa(samples)
	}
	return []string{"-m", "tcp", "-x", "-s", "1", "-L", l}
}

// Sample runs nettop for `seconds` seconds and returns one Snapshot whose rates are the
// average throughput over the window (first frame diffed against the last). If res is
// non-nil, remote hostnames are resolved.
func Sample(ctx context.Context, seconds int, res *Resolver) (Snapshot, error) {
	if seconds < 1 {
		seconds = 1
	}
	out, err := exec.CommandContext(ctx, "nettop", nettopArgs(seconds+1)...).Output()
	if err != nil {
		return Snapshot{}, fmt.Errorf("nettop: %w", err)
	}
	snap, ok := snapshotFromReader(bytes.NewReader(out), float64(seconds), realClock{})
	if !ok {
		return Snapshot{}, fmt.Errorf("nettop produced no samples")
	}
	if res != nil {
		res.Annotate(&snap)
	}
	return snap, nil
}

// Stream runs nettop continuously and sends a Snapshot (diffed against the previous
// frame, ~1s cadence) to out until ctx is cancelled or nettop exits.
func Stream(ctx context.Context, res *Resolver, out chan<- Snapshot) error {
	cmd := exec.CommandContext(ctx, "nettop", nettopArgs(0)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait()

	fsr := newFrameScanner(stdout)
	var prev *frame
	for {
		f, ok := fsr.next()
		if !ok {
			break
		}
		if prev != nil {
			snap := diff(prev, f, 1, realClock{})
			if res != nil {
				res.Annotate(&snap)
			}
			select {
			case out <- snap:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		prev = f
	}
	return nil
}

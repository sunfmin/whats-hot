// Command netmon is the network surface of the whats-hot skill: a live network monitor,
// a machine-readable snapshot for Claude, a self-resuming download client, and a link
// speed test. See ../../docs/CONTEXT.md and ../../docs/adr/ for the design.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/sunfmin/whats-hot/internal/dashboard"
	"github.com/sunfmin/whats-hot/internal/deep"
	"github.com/sunfmin/whats-hot/internal/download"
	"github.com/sunfmin/whats-hot/internal/monitor"
	"github.com/sunfmin/whats-hot/internal/speedtest"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "snapshot":
		err = cmdSnapshot(args)
	case "serve":
		err = cmdServe(args)
	case "get":
		err = cmdGet(args)
	case "speedtest":
		err = cmdSpeedtest(args)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "netmon: unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "netmon:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `netmon — live network activity, snapshots, downloads, speed test

USAGE:
  netmon snapshot [-seconds N] [-deep] [-all] [-no-resolve]   sample activity -> JSON (for Claude)
  netmon serve    [-addr host:port] [-no-open]                live web dashboard (for humans)
  netmon get <url> [-o file] [-retries N] [-stall-window D]   download with auto-resume
  netmon speedtest                                            measure link capacity (networkQuality)

Monitoring is read-only and needs no sudo. -deep adds TLS-SNI hostnames via packet
capture (needs sudo). get writes only the file you ask for.
`)
}

// signalContext cancels on Ctrl-C / SIGTERM for clean shutdown.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func cmdSnapshot(args []string) error {
	fs := newFlagSet("snapshot")
	seconds := fs.Int("seconds", 3, "sampling window in seconds")
	deepMode := fs.Bool("deep", false, "resolve exact hostnames via TLS SNI (needs sudo)")
	all := fs.Bool("all", false, "include idle processes (default: only active)")
	noResolve := fs.Bool("no-resolve", false, "skip reverse-DNS hostname lookup")
	compact := fs.Bool("compact", false, "compact JSON (default: indented)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, cancel := signalContext()
	defer cancel()

	var res *monitor.Resolver
	if !*noResolve {
		res = monitor.NewResolver()
	}

	// In -deep mode, capture SNI concurrently with the sampling window.
	sniCh := make(chan map[string]string, 1)
	if *deepMode {
		go func() {
			m, err := deep.Resolve(ctx, *seconds)
			if err != nil {
				fmt.Fprintln(os.Stderr, "netmon: deep SNI capture failed:", err)
			}
			sniCh <- m
		}()
	}

	snap, err := monitor.Sample(ctx, *seconds, res)
	if err != nil {
		return err
	}
	if *deepMode {
		applySNI(&snap, <-sniCh)
	}
	if !*all {
		snap = activeOnly(snap)
	}

	enc := json.NewEncoder(os.Stdout)
	if !*compact {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(snap)
}

func cmdServe(args []string) error {
	fs := newFlagSet("serve")
	addr := fs.String("addr", "127.0.0.1:7345", "listen address")
	noOpen := fs.Bool("no-open", false, "do not open the browser")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, cancel := signalContext()
	defer cancel()

	l, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	url := "http://" + l.Addr().String()
	fmt.Fprintf(os.Stderr, "\nnetmon dashboard: %s  (Ctrl-C to stop)\n", url)
	if !*noOpen {
		_ = exec.Command("open", url).Start()
	}
	return dashboard.New().Serve(ctx, l)
}

func cmdGet(args []string) error {
	fs := newFlagSet("get")
	out := fs.String("o", "", "output file (default: derived from URL)")
	retries := fs.Int("retries", 5, "resume attempts after a stall")
	stallWindow := fs.Duration("stall-window", 10*time.Second, "time below threshold before resuming")
	stallBps := fs.Int64("stall-bps", 1024, "bytes/sec below which a transfer counts as stalled")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Allow flags on either side of the URL: flag stops at the first positional, so pull
	// the URL out and re-parse whatever trailed it (e.g. `get <url> -o file`).
	rest := fs.Args()
	if len(rest) == 0 {
		return fmt.Errorf("usage: netmon get <url> [-o file]")
	}
	url := rest[0]
	if len(rest) > 1 {
		if err := fs.Parse(rest[1:]); err != nil {
			return err
		}
	}
	ctx, cancel := signalContext()
	defer cancel()

	start := time.Now()
	res, err := download.Get(ctx, download.Options{
		URL: url, Dest: *out, MaxRetries: *retries,
		StallWindow: *stallWindow, StallBytesPerSec: *stallBps,
	}, progressPrinter())
	fmt.Fprintln(os.Stderr) // end progress line
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "saved %s (%s) in %s across %d attempt(s)%s\n",
		res.Path, humanBytes(res.Bytes), time.Since(start).Round(time.Millisecond),
		res.Attempts, resumedNote(res.Resumed))
	return nil
}

func cmdSpeedtest(args []string) error {
	fs := newFlagSet("speedtest")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	fmt.Fprintln(os.Stderr, "running networkQuality (this takes ~15s)…")
	r, err := speedtest.Run(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "↓ download: %s   ↑ upload: %s   responsiveness: %.0f RPM\n",
		humanBits(r.DownlinkBps), humanBits(r.UplinkBps), r.Responsiveness)
	return json.NewEncoder(os.Stdout).Encode(r)
}

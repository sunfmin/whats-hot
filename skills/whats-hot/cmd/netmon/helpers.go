package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sunfmin/whats-hot/internal/download"
	"github.com/sunfmin/whats-hot/internal/monitor"
)

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

// activeOnly drops idle processes (no current throughput and no Established flow), and
// within each kept process drops non-Established flows — so the output is only what is
// actually transferring.
func activeOnly(snap monitor.Snapshot) monitor.Snapshot {
	out := snap
	out.Processes = nil
	for _, p := range snap.Processes {
		var flows []monitor.Flow
		for _, f := range p.Flows {
			if f.State == "Established" {
				flows = append(flows, f)
			}
		}
		if p.DownBps == 0 && p.UpBps == 0 && len(flows) == 0 {
			continue
		}
		p.Flows = flows
		out.Processes = append(out.Processes, p)
	}
	return out
}

// applySNI overlays exact TLS-SNI hostnames (from --deep) onto flow endpoints, preferring
// them over reverse-DNS names.
func applySNI(snap *monitor.Snapshot, sni map[string]string) {
	if len(sni) == 0 {
		return
	}
	for pi := range snap.Processes {
		for fi := range snap.Processes[pi].Flows {
			fl := &snap.Processes[pi].Flows[fi]
			if h, ok := sni[fl.Remote.IP]; ok && h != "" {
				fl.Remote.Host = h
			}
		}
	}
}

// progressPrinter returns a download progress callback that redraws a single stderr line.
func progressPrinter() download.ProgressFunc {
	return func(downloaded, total int64) {
		if total > 0 {
			pct := float64(downloaded) / float64(total) * 100
			fmt.Fprintf(os.Stderr, "\r  %s / %s (%.0f%%)      ",
				humanBytes(downloaded), humanBytes(total), pct)
			return
		}
		fmt.Fprintf(os.Stderr, "\r  %s downloaded      ", humanBytes(downloaded))
	}
}

func resumedNote(resumed bool) string {
	if resumed {
		return " (resumed)"
	}
	return ""
}

func humanBytes(n int64) string { return human(float64(n), []string{"B", "KB", "MB", "GB", "TB"}) }

func humanBits(bitsPerSec float64) string {
	return human(bitsPerSec, []string{"bps", "Kbps", "Mbps", "Gbps"})
}

func human(v float64, units []string) string {
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

---
name: whats-hot
description: Investigates macOS resource hogs — CPU, memory, disk I/O, GPU — AND network activity, and explains what is happening. For hotness it captures sample(1), vmmap, fs_usage, powermetrics traces to find hot code paths and root causes. For the network it runs the bundled `netmon` tool to show live upload/download speed, which app/process is using the network, per-connection remote IPs and hostnames, a link speed test, and resumable downloads. Use when the user says "what's hot on my mac", "what's using my CPU", "why is my Mac slow / hot / fans loud", "find the process eating RAM", "profile PID", "why is <app> busy" — and for "network / upload / download speed", "what's using my network", "which app is connecting where", "what IPs am I connected to", "watch my network", "a download is stuck / resume it", or "run a speed test".
---

# What's Hot (macOS)

Two jobs: (1) find resource hogs and explain **why** they are hot via traces; (2) show and
query **network activity** — throughput, connections, downloads — via the `netmon` tool.

## Safety rules — non-negotiable

- **Observation is read-only.** When investigating or monitoring, never `kill`/`pkill`/
  `renice`/`launchctl unload` and never touch the processes you observe. Recommend, ask the user.
- **Downloads are the one explicit write.** `netmon get <url>` is a user-initiated action that
  writes only the file the user asked for. It never reaches into another app's transfer — the
  monitor cannot resume Safari/Chrome/App Store downloads (see docs/adr/0001); it can only observe them.
- **No silent sudo.** `fs_usage`, `powermetrics`, `vmmap` of other-user PIDs, and `netmon -deep`
  (packet capture) need root. State the command, state why, wait for OK. If `~/.claude/CLAUDE.md`
  defines a `SUDO_ASKPASS` protocol, use it.
- **One PID per investigation.** Multi-PID apps (Chrome, Slack, Electron): list first, pick the hot one.

## Which job?

- "why is <app>/my mac hot / slow / fans loud / eating RAM / using CPU / GPU" → **Hotness workflow**.
- "network / up/down speed, what's using my network, which app connects where, what IPs, watch the
  network, a download is stuck, resume a download, speed test" → **Network workflow**.

## Hotness workflow

1. **Triage** — `bash scripts/top-offenders.sh`. No sudo.
2. **Pick PID.** Named app → `pgrep -lf <name>`, match to step 1.
3. **CPU profile** — `bash scripts/profile-pid.sh <pid> [secs]` (default 5). No sudo for own PIDs.
4. **Hot frames** — `bash scripts/summarize-sample.sh <sample-file>`.
5. **Memory** — `bash scripts/mem-breakdown.sh <pid>`.
6. **I/O + net** (if CPU+mem don't explain) — `bash scripts/io-net-snapshot.sh <pid> [secs]`. For
   richer per-connection network detail, use `netmon snapshot` (below).
7. **GPU** (graphical/ML/video only) — `bash scripts/gpu-snapshot.sh [secs]`. Sudo.
8. **Conclude.** 3–6 line summary (template below).

## Network workflow (netmon)

`netmon` is a Go tool bundled with this skill. **Build it once** (needs Go):
`bash scripts/build-netmon.sh` → installs `~/.local/bin/netmon`. The diagnostic scripts above do
not need it; only these network features do.

Drive it by intent — **always use `snapshot` when reporting to the user** (the dashboard is for the
human at the browser, not for you):

- **"what's using my network / up-down speed / what IPs / which app connects where"** →
  `netmon snapshot -seconds 3` → JSON: machine total up/down bytes-per-sec, per-process throughput,
  and per-connection Flows (remote IP, reverse-DNS host, port, protocol, state). Report the top
  talkers and their remote endpoints. Add `-all` to include idle processes; `-no-resolve` to skip DNS.
- **Exact hostname / see plaintext HTTP URLs** → `netmon snapshot -deep` (adds TLS-SNI hostnames via
  packet capture — **needs sudo**; state the command and wait for OK).
- **"pop up a live view / watch my network"** → tell the user to run `netmon serve` themselves; it
  opens a live web dashboard in their browser. Do not run it yourself — you cannot read a live view.
- **"a download is stuck / download this and don't let it stall"** → `netmon get <url> [-o file]`.
  Auto-resumes on a stall via HTTP range requests. For a stuck download in *another* app, netmon can
  only observe it — tell the user to retry it there.
- **"how fast is my connection / run a speed test"** → `netmon speedtest` (wraps networkQuality; ~15s).

A **Remote Endpoint is an IP + hostname + port + protocol, never a full URL** — HTTPS encrypts the
path. See docs/CONTEXT.md for the vocabulary and docs/adr/ for the design decisions.

## Reporting template (hotness)

```
PID 1234 — Google Chrome Helper (Renderer)
  CPU: 187%  Memory: 2.4 GB RSS  I/O: idle  GPU: n/a
  Hot path (62% of samples): blink::Document::updateStyle ← StyleResolver::resolveStyle
  Likely cause: heavy CSS recalc, probably in one tab.
  Next step: ask the user to identify the tab; suggest closing it.
```

Trace inconclusive (mostly `mach_msg_trap`) → say so. No invented cause.

## Trace interpretation

See [REFERENCE.md](REFERENCE.md): tool catalog, hot-path patterns, CPU vs wait vs I/O.

## NOT this skill

- Reproducing perf bugs in your own code → `diagnose` skill.
- Fixing an offender → this skill investigates and recommends; it does not kill or restart.
- Persisting network history for later querying → netmon is point-in-time by design (docs/adr/0003).

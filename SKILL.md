---
name: whats-hot
description: Investigates macOS apps consuming excessive CPU, memory, disk I/O, network, or GPU and explains why they are hot. Captures sample(1), vmmap, fs_usage, nettop, and powermetrics traces to identify hot code paths and root causes. Use when the user says "what's hot on my mac", "what's using my CPU", "why is my Mac slow / hot / fans spinning / loud", "find the process eating RAM", "what is <app> doing", "profile this process", "battery draining", "fan is loud", or asks why a specific PID is busy.
---

# What's Hot (macOS)

Goal: find top consumers, explain **why** hot via traces.

## Safety rules — non-negotiable

- **Read-only.** Never `kill`/`pkill`/`renice`/`launchctl unload`. Recommend, ask user.
- **No silent sudo.** `fs_usage`, `powermetrics`, `vmmap` of other-user PIDs need root. State cmd, state reason, wait for OK. If `~/.claude/CLAUDE.md` defines `SUDO_ASKPASS` protocol, use that.
- **One PID per investigation.** Multi-PID apps (Chrome, Slack, Electron): list first, pick hot one.

## Workflow

1. **Triage** — `bash scripts/top-offenders.sh`. No sudo.
2. **Pick PID.** Named app -> `pgrep -lf <name>`, match to step 1.
3. **CPU profile** — `bash scripts/profile-pid.sh <pid> [secs]` (default 5). No sudo for own PIDs.
4. **Hot frames** — `bash scripts/summarize-sample.sh <sample-file>`.
5. **Memory** — `bash scripts/mem-breakdown.sh <pid>`.
6. **I/O + net** (if CPU+mem don't explain) — `bash scripts/io-net-snapshot.sh <pid> [secs]`. `fs_usage` sudo; `nettop` no.
7. **GPU** (graphical/ML/video only) — `bash scripts/gpu-snapshot.sh [secs]`. Sudo.
8. **Conclude.** 3–6 line summary, template below.

## Reporting template

```
PID 1234 — Google Chrome Helper (Renderer)
  CPU: 187%  Memory: 2.4 GB RSS  I/O: idle  GPU: n/a
  Hot path (62% of samples): blink::Document::updateStyle ← StyleResolver::resolveStyle
  Likely cause: heavy CSS recalc, probably in one tab.
  Next step: ask the user to identify the tab; suggest closing it.
```

Trace inconclusive (mostly `mach_msg_trap`) -> say so. No invented cause.

## Trace interpretation

See [REFERENCE.md](REFERENCE.md): tool catalog, hot-path patterns, CPU vs wait vs I/O.

## NOT this skill

- Long-term monitoring -> Activity Monitor / `system-monitor` skill.
- Repro perf bugs in own code -> `diagnose` skill.
- Fix offender -> investigation + recommendation only.

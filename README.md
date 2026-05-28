# whats-hot

A Claude Code skill that investigates macOS apps consuming excessive CPU,
memory, disk I/O, network, or GPU — and explains **why** they are hot.

It is a read-only, ask-before-sudo workflow that captures `sample(1)`, `vmmap`,
`fs_usage`, `nettop`, and `powermetrics` traces, then summarises the hot code
paths and likely root cause.

## Install

With [vercel-labs/skills](https://github.com/vercel-labs/skills):

```sh
npx skills add sunfmin/whats-hot
```

Or manually — clone into your skills directory:

```sh
git clone https://github.com/sunfmin/whats-hot ~/.claude/skills/whats-hot
```

## Use

In Claude Code, ask one of:

- "what's hot on my mac"
- "what's using my CPU"
- "why are my fans loud"
- "find the process eating RAM"
- "profile PID 1234"
- "why is `<app>` busy"

Claude will run the bundled scripts to triage, profile, summarise hot frames,
and break down memory. For commands that need root (`fs_usage`,
`powermetrics`, `vmmap` of other-user PIDs), it states the command and waits
for your OK.

## Workflow

1. **Triage** — `scripts/top-offenders.sh` lists top CPU and RSS consumers.
2. **CPU profile** — `scripts/profile-pid.sh <pid> [secs]` runs `sample(1)`.
3. **Hot frames** — `scripts/summarize-sample.sh <file>` extracts leaves.
4. **Memory** — `scripts/mem-breakdown.sh <pid>` (vmmap + footprint).
5. **I/O + network** — `scripts/io-net-snapshot.sh <pid> [secs]` if needed.
6. **GPU** — `scripts/gpu-snapshot.sh [secs]` if the app is graphical / ML.

See [SKILL.md](SKILL.md) for the full protocol and [REFERENCE.md](REFERENCE.md)
for tool catalog, hot-path patterns, and CPU-bound vs wait-bound interpretation.

## Sudo without a tty

Claude Code runs `sudo` in a shell with no terminal, so the password prompt
has nowhere to go. Options:

- A narrow `NOPASSWD` sudoers entry for `sample`, `vmmap`, `footprint`,
  `fs_usage`, `powermetrics`, `spindump`.
- [`sudoplz`](https://github.com/vercel-labs/skills) — `SUDO_ASKPASS` helper
  that pops a GUI approval dialog per command.
- Run sudo commands yourself in a separate terminal and paste output back.

If `~/.claude/CLAUDE.md` defines a `SUDO_ASKPASS` protocol, the skill uses it.

## Not in scope

- Long-term monitoring → use Activity Monitor or the `system-monitor` skill.
- Reproducing perf bugs in your own code → use the `diagnose` skill.
- Killing or restarting offenders — the skill only investigates and recommends.

## License

MIT

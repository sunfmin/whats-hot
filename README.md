# whats-hot

A Claude Code skill for macOS that does two things:

1. **Resource hogs** — investigates apps consuming excessive CPU, memory, disk I/O, or
   GPU and explains **why** they are hot, by capturing `sample(1)`, `vmmap`, `fs_usage`,
   and `powermetrics` traces and summarising the hot code paths.
2. **Network activity** — shows and queries live upload/download speed, which app/process
   is using the network, per-connection remote IPs and hostnames, runs a link speed test,
   and downloads files with automatic resume — via the bundled `netmon` tool.

Observation is read-only and needs no sudo. Downloads (`netmon get`) are an explicit,
user-initiated action that writes only the requested file.

## Install

```sh
npx skills add sunfmin/whats-hot
```

The diagnostic scripts work out of the box. The **network features need the `netmon`
binary**, which is built from Go source bundled with the skill (Go is required):

```sh
bash ~/.claude/skills/whats-hot/scripts/build-netmon.sh   # -> ~/.local/bin/netmon
```

Ensure `~/.local/bin` is on your `PATH`.

## Use

In Claude Code, ask one of:

**Hotness**
- "what's hot on my mac" · "what's using my CPU" · "why are my fans loud"
- "find the process eating RAM" · "profile PID 1234" · "why is `<app>` busy"

**Network**
- "what's using my network" · "what's my upload/download speed right now"
- "which app is connecting, and to what IPs?" · "watch my network" (opens a live dashboard)
- "this download is stuck — resume it" · "download `<url>` and don't let it stall"
- "how fast is my internet / run a speed test"

## netmon commands

| Command | What it does | For |
| --- | --- | --- |
| `netmon snapshot [-seconds N] [-deep] [-all]` | Sample activity → JSON: total + per-process + per-connection throughput and remote endpoints | Claude / scripts |
| `netmon serve [-addr host:port]` | Live web dashboard (htmlgo + SSE), opens in the browser | Humans |
| `netmon get <url> [-o file]` | Download with HTTP-range auto-resume on stall | Both |
| `netmon speedtest` | Link capacity via `networkQuality` (~15s) | Both |

`-deep` resolves exact hostnames from the TLS SNI via packet capture (needs sudo). A
remote endpoint is an IP + hostname + port + protocol — never a full URL, since HTTPS
encrypts the path.

See [SKILL.md](SKILL.md) for the full protocol, [docs/CONTEXT.md](docs/CONTEXT.md) for the
domain vocabulary, [docs/adr/](docs/adr) for the design decisions, and
[REFERENCE.md](REFERENCE.md) for trace interpretation.

## Sudo without a tty

Claude Code runs `sudo` with no terminal, so the password prompt has nowhere to go.
`fs_usage`, `powermetrics`, `vmmap` of other-user PIDs, and `netmon -deep` need root. Options:

- A narrow `NOPASSWD` sudoers entry for `sample`, `vmmap`, `footprint`, `fs_usage`,
  `powermetrics`, `spindump`, `tcpdump`.
- [`sudoplz`](https://github.com/vercel-labs/skills) — a `SUDO_ASKPASS` helper that pops a
  GUI approval dialog per command.
- Run the sudo command yourself in a separate terminal and paste the output back.

## Not in scope

- Reproducing perf bugs in your own code → use the `diagnose` skill.
- Killing or restarting offenders — the skill investigates and recommends only.
- Persisting network history for later querying — `netmon` is point-in-time by design
  ([ADR-0003](docs/adr/0003-no-persistence.md)); it keeps only an in-memory rolling window.

## License

MIT

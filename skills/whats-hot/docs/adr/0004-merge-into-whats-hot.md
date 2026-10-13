# Merge network-monitor into the whats-hot skill

The network-monitor capability (live network dashboard + JSON snapshot + managed downloads with auto-resume + speed test) is delivered as an expansion of the existing published `whats-hot` skill rather than a standalone skill. The Go tool lives at `whats-hot/cmd/netmon`, these design docs move to `whats-hot/docs/`, and `~/Developments/network-monitor` is retired.

This was chosen over keeping two cross-referenced skills (the recommended alternative) to avoid a second "network/perf on my Mac" skill. Consequences we accepted:

1. whats-hot's "Read-only, never mutate" rule is **reframed** — observation and diagnosis stay strictly read-only (never kill/renice/unload an observed process), while download/resume is an explicit user-initiated action (`netmon get`) that writes only the requested target file.
2. whats-hot's "no long-term monitoring" scope boundary is **relaxed** to admit the live dashboard.
3. whats-hot is **no longer zero-build** — it now requires Go and builds `netmon` at install time.
4. Its description/triggers **broaden** to cover network speed, activity, per-connection endpoints, and stuck downloads/resume.

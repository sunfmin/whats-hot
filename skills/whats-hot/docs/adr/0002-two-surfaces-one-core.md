# Two surfaces over one Go core: a snapshot CLI for Claude, a live web dashboard for humans

The tool serves two audiences that need opposite output shapes. Claude drives a machine-readable **snapshot** command (sample N seconds → JSON → exit) because it cannot watch a continuously refreshing view. A human wants a live, always-updating view. We deliver the human surface as a local Go HTTP server that renders the page with `htmlgo` and pushes updates over **Server-Sent Events** (a small amount of client JS applies them); running the tool opens it in the browser. Both surfaces call the same in-process Go core (sampling, DNS resolution, download/resume), so sampling logic has a single home.

Everything stays pure Go — no CGo, Node, or Xcode — so the whole thing builds with `go build`. Rejected: a native GUI (Fyne / Wails / SwiftUI), heavier to build and distribute for a personal tool, when htmlgo + SSE gives a richer dashboard with less code.

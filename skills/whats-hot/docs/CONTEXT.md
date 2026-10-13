# Network Monitor

A macOS command-line tool — and the Claude skill that drives it — for seeing, in real time, all network activity on the machine (who is transferring, how fast, to which remote endpoint), running an on-demand link speed test, and running a self-managed download client that can auto-resume stalled transfers.

## Language

**Throughput**:
The bytes-per-second currently flowing up and/or down — actual usage happening right now. This is what "速度" means in the live monitor.
_Avoid_: bandwidth, speed

**Link Capacity**:
The benchmarked maximum the internet connection can sustain (down/up Mbps + latency), measured on demand. Distinct from Throughput — one is current usage, the other is the pipe's ceiling.
_Avoid_: speed, bandwidth

**Flow**:
A single active network connection owned by one process — one socket to one remote endpoint.
_Avoid_: connection (ok informally), session, stream

**Remote Endpoint**:
The identity of the far side of a Flow: IP address, reverse-DNS (or SNI) hostname, port, and protocol. It is NOT a full URL — HTTPS encrypts the path, so no `/path?query` is recoverable.
_Avoid_: download link, URL, address

**Managed Download**:
A transfer the tool initiates itself. Because the tool owns it, it can retry and auto-resume it. Contrast with an Observed Transfer.
_Avoid_: our download, internal download

**Observed Transfer**:
Any transfer belonging to another app (Safari, Chrome, App Store, curl…) that the monitor can watch but not control. The tool can observe it but cannot resume it.
_Avoid_: external download, third-party download

**Stall**:
The condition where a transfer's Throughput stays at approximately zero while its Flow remains open, for longer than a threshold duration. Triggers an auto-resume for a Managed Download; for an Observed Transfer the tool takes no action beyond showing it in the live view.
_Avoid_: hang, freeze, stuck (ok informally)

**Snapshot**:
The machine-readable mode — sample network activity for a fixed window, emit one structured JSON result, then exit. The surface Claude drives.
_Avoid_: report, dump

**Dashboard**:
The human-facing live view — a local HTTP page rendered with htmlgo and updated over Server-Sent Events. The surface a person opens in the browser.
_Avoid_: UI, GUI, window

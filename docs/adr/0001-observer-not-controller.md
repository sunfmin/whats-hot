# The monitor observes; auto-resume is scoped to tool-managed downloads

macOS provides no API to control another application's in-flight transfer, so a network monitor cannot resume a stalled download owned by Safari, Chrome, the App Store, or an arbitrary process. We therefore split the tool in two: a read-only **Monitor** that watches all system network activity; and a self-contained **Managed Download** client (the tool's own download command) that owns the transfers it starts and can genuinely auto-resume them.

"Auto-resume any stuck download on the system" was rejected as impossible — the only paths to it (a MITM proxy or a Network Extension intercepting other apps' traffic) are fragile, permission-heavy, and out of scope.

## Amendment (2026-07-10)

The Monitor originally raised a macOS **Alert** (Notification Center banner) when it detected a Stall on an Observed Transfer. That notification feature was removed — the live dashboard surfaces stalls visually and the banners added noise without adding a control the tool could offer. The Monitor is now purely observational: it shows activity and takes no action on it.

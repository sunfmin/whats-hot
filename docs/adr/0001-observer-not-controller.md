# The monitor observes; auto-resume is scoped to tool-managed downloads

macOS provides no API to control another application's in-flight transfer, so a network monitor cannot resume a stalled download owned by Safari, Chrome, the App Store, or an arbitrary process. We therefore split the tool in two: a read-only **Monitor** that watches all system network activity and, on detecting a Stall, can only raise an **Alert**; and a self-contained **Managed Download** client (the tool's own download command) that owns the transfers it starts and can genuinely auto-resume them.

"Auto-resume any stuck download on the system" was rejected as impossible — the only paths to it (a MITM proxy or a Network Extension intercepting other apps' traffic) are fragile, permission-heavy, and out of scope.

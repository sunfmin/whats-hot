# Reference: macOS resource investigation

## Tool catalog

| Tool | Shows | Sudo? | Notes |
|------|-------|-------|-------|
| `ps -Ao pid,pcpu,pmem,rss,comm` | CPU%/mem%/RSS snapshot | no | %CPU = lifetime avg, not "now". |
| `top -l 2 -s 1 -o cpu` | Live CPU% | no | First sample = lifetime avg, skip. |
| `sample <pid> <secs>` | User stacks ~1ms | no (own PIDs) | Workhorse. "Sort by top of stack" = hot leaves. |
| `spindump <pid> <secs>` | Kernel+user stacks | yes | Heavier. Use when `sample` shows syscall stuck. |
| `vmmap -summary <pid>` | VM region map | sometimes | MALLOC / MALLOC_LARGE / Stack / VM_ALLOCATE. |
| `footprint -pid <pid>` | Mem accounting (compressed/dirty/swapped) | no | Needs Xcode CLT. |
| `heap <pid>` | Heap class breakdown | no | ObjC/Swift/CF only. |
| `leaks <pid>` | Leaked allocs | no | Slow. Only if mem grows. |
| `fs_usage -w <pid>` | Every file syscall | yes | Verbose. Filter `-f filesys` / `-f diskio`. |
| `nettop -P -p <pid>` | Per-PID sockets | no | `-L 1` snapshot, `-L N` loop. |
| `lsof -p <pid>` | Open FDs | no | What's open now. |
| `powermetrics --samplers gpu_power,tasks` | GPU + per-task energy | yes | Apple Silicon: P-core/E-core split. |
| `iostat 1 5` | Disk throughput | no | System-wide, not per-PID. |

## Reading `sample(1)`

Sections top to bottom:

1. **Header** — params + PID.
2. **Call graph** — per thread, top-down. E.g.:
   ```
   2500 Thread_1234   DispatchQueue_1: com.apple.main-thread
   + 2500 start  (in libdyld.dylib) ...
   +   2500 main  (in MyApp) ...
   +     2400 -[ViewController doWork]  (in MyApp) ...
   +       2200 -[Parser parseChunk:]  (in MyApp) ...
   +         2100 objc_msgSend  (in libobjc.A.dylib) ...
   ```
   Number = sample count. Indent = depth. Child ≈ parent count -> parent waits on child.
3. **Sort by top of stack** — leaves only, by count. **Most actionable.** `scripts/summarize-sample.sh` extracts this.
4. **Binary Images** — libs.

### Rules of thumb

- Same leaf >30% samples -> hot spot.
- Spread thin -> varied work or too-short sample. Rerun `seconds=20`.
- All in `mach_msg_trap`/`__semwait_signal`/`__psynch_cvwait`/`kevent` -> **waiting**, not computing. CPU% high + stack idle -> wakeup storm. Look at parent frame driving wakeups.

## Hot-path patterns

| Pattern | Meaning |
|---------|---------|
| `mach_msg_trap` deep | IPC spin (XPC, distributed notifs). Look 2-3 frames up. |
| `__semwait_signal` | Lock contention. `spindump` to find holder. |
| `objc_msgSend` hot leaf | Cocoa hot loop. Real spot = caller. |
| `gc_collect_main` / `mark_phase` / `JSC::Heap::collect` | GC pressure. Mem grew before spike. |
| `CA::Render::*`, `CA::Display::DisplayLink::dispatch` | Animation/compositor. Safe unless continuous. |
| `Metal::*Encoder::*`, `IOAccelCommandQueue` heavy | GPU submission. Pair with `gpu-snapshot.sh`. |
| `read`/`pread`/`write`/`pwrite` | I/O-bound. `io-net-snapshot.sh`. |
| `recvfrom`/`sendto`/`select`/`kevent_qos` (+ net FDs in lsof) | Net-bound. `nettop`. |
| `__vm_page_fault` heavy | Mem pressure -> paging. Check `vm_stat` swap. |
| `malloc_zone_malloc`/`free_tiny` | Alloc churn. `heap <pid>`. |
| Many short stacks, many threads | Thread thrash / pool too large. |

## Reading `vmmap -summary`

Watch:
- **MALLOC_LARGE / MALLOC_HUGE** big -> real heap growth. `heap <pid>`.
- **VM_ALLOCATE (reserved)** big + committed small -> reserved space, not RAM. Harmless.
- **Mapped File** big -> mmap'd (SQLite/LevelDB/model weights). Not RAM unless touched.
- **Stack** >8MB/thread -> thread leak or recursion.

## CPU vs wait vs I/O

| Top leaves | Tool | Category |
|------------|------|----------|
| User code | `summarize-sample.sh` | CPU-bound |
| `__*wait*` / `mach_msg_trap` | `spindump` | Wait-bound (lock/IPC) |
| `read`/`write`/`pread` | `io-net-snapshot.sh` | I/O-bound |
| `recvfrom`/`sendto` | `nettop` | Net-bound |
| `top` high CPU% + `sample` idle stacks | Check thread count | Wakeup storm |

## When sample isn't enough

- Sandboxed (App Store) -> `sample` may fail silent. Try `sudo sample`.
- JIT (Node/Chrome/JVM) -> symbols `???`. Use runtime profiler (`node --prof`, devtools, async-profiler).
- Mostly in kernel -> `spindump`.
- Spawn/exit storm -> `dtrace -n 'proc:::exec-success { ... }'` (sudo + SIP off or signed).

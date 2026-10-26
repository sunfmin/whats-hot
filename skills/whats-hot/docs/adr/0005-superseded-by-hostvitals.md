# whats-hot is superseded by hostvitals

hostvitals now provides whats-hot's host and process observations. whats-hot
becomes a pointer to hostvitals: install its CLI and skill, then follow
`hostvitals --help`. This supersedes ADR-0004's decision to bundle netmon here.

The scripts, netmon Go module, REFERENCE.md and CONTEXT.md are removed. The
earlier ADRs remain as the record of the retired implementation's decisions.

| Retired entry point | Replacement |
| --- | --- |
| `top-offenders.sh` | `hostvitals now` |
| `profile-pid.sh`, `summarize-sample.sh` | `hostvitals profile` |
| `mem-breakdown.sh` | `hostvitals memory` |
| `io-net-snapshot.sh` | `hostvitals process` for open files, sockets and, as root, files read and written; `hostvitals now` for network Throughput |
| `gpu-snapshot.sh` | `hostvitals now` for power and, as root, per-process GPU time |
| `netmon snapshot` | `hostvitals now` and `hostvitals process` |

By decision, the Dashboard, Managed Downloads, Link Capacity test and
`snapshot -deep` are not carried over. No Go build or bundled scripts are
needed for the pointer skill.

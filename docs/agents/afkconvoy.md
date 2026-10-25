# afkconvoy config

How the afkconvoy fleet works this repo's backlog: issues labelled
`ready-for-agent`, one worker each, landed on `main` behind `./gate.sh`.

```yaml
base_branch: "main"
gate_command: ./gate.sh
gate_seconds: 900
ready_label: ready-for-agent
```

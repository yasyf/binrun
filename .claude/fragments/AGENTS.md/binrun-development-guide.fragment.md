# binrun Development Guide

Fetch, verify, and exec the exact artifact a descriptor pins — release binaries, Python tools, signed apps. Distributed via Homebrew: `brew install yasyf/tap/binrun`.

## Repository Structure

```
binrun/
├── cmd/binrun/   # main package — argv routing and error→exit-code mapping
├── internal/
│   ├── cli/               # transparent exec path + verbs behind --: fetch, resolve, parse, latest, gc, cache-dir
│   ├── version/           # build version, stamped via -ldflags
│   └── log/               # slog setup
├── descriptor/            # binrun's own release descriptor template
├── scripts/               # render-descriptor.sh — fills the descriptor from a goreleaser dist tree
├── docs/examples/         # runnable example descriptors the README demonstrates
├── .github/               # GitHub Actions workflows
├── AGENTS.md              # This file — shared conventions
└── README.md              # Project overview
```

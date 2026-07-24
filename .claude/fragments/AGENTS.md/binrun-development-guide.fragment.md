# binrun Development Guide

Fetch, verify, and exec the exact artifact a descriptor pins — release binaries, Python tools, signed apps. Distributed via Homebrew: `brew install yasyf/tap/binrun`.

## Repository Structure

```
binrun/
├── cmd/binrun/   # main package — the CLI entry point
├── internal/
│   ├── cli/               # cobra command tree — TODO(bootstrap): name the commands
│   ├── version/           # build version, stamped via -ldflags
│   └── log/               # slog setup
├── .github/               # GitHub Actions workflows
├── AGENTS.md              # This file — shared conventions
└── README.md              # Project overview
```

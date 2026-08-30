# Changelog

All notable changes to this project are documented here.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.1] - 2026-08-29

### Changed
- Repinned daemonkit to v0.23.0 (from v0.21.0). Every daemonkit package binrun
  compiles against is unchanged between these versions, so no command, flag, or
  on-disk path moved. The release republishes the descriptor and the Homebrew
  cask against the current daemonkit.

## [0.3.0] - 2026-08-03

### Changed
- Repinned daemonkit to v0.21.0 (from v0.17.2). binrun uses only the portable
  subset: `artifact`, `bundle`, `durable`, `ghrelease`, and `version`. No
  command or flag changed.
- The content cache directory now resolves your home directory from the passwd
  database instead of `$HOME`, so a sandboxed or overridden `HOME` no longer
  moves it. Set `DAEMONKIT_HOME` to put binrun's state somewhere else; daemonkit
  logs a warning when you do.

## [0.2.0] - 2026-07-24

### Added
- Transparent exec: `binrun FILE [args…]` resolves a descriptor via the
  `daemonkit/artifact` store and execs the pinned artifact, forwarding args
  untouched (direct shebang invocation works the same way). The artifact's exit
  code becomes binrun's.
- Management verbs behind a `--` separator: `fetch` (pre-warm), `resolve`
  (print the local path), `parse` (print normalized descriptor JSON), `latest`
  (print the descriptor repo's latest release tag), `gc --keep N` (prune the
  content cache, keeping the newest N materializations per artifact name), and
  `cache-dir` (print the content cache directory).
- Exit discipline: every runner-domain failure exits 1 with a terse stderr
  message (artifact sentinels mapped to human strings; a `ManualUpgradeError`
  renders its `brew upgrade --cask` handoff). binrun never exits 2 — that code
  is reserved for hook verdicts — and the only other codes come from the
  exec'd artifact.
- Own descriptor: `descriptor/binrun.binrun.tmpl` plus
  `scripts/render-descriptor.sh`, which fills the version and per-platform
  size/digest/asset-name from a goreleaser dist tree. A follow-on release job
  renders it and uploads it to each release.

[0.3.1]: https://github.com/yasyf/binrun/releases/tag/v0.3.1
[0.3.0]: https://github.com/yasyf/binrun/releases/tag/v0.3.0
[0.2.0]: https://github.com/yasyf/binrun/releases/tag/v0.2.0

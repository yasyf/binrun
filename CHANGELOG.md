# Changelog

All notable changes to this project are documented here.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.8.1] - 2026-09-20

### Fixed

- `binrun -- gc` now applies the tool store's live-process guard to cached
  release binaries too. On this machine, `--keep 2` would have deleted 52 of
  66 cache entries, including a running `ccx` and `codex-ask`: the fourth and
  fifth newest entries under their names. Long-running MCP servers and
  daemons can use older binaries for days. Removing their directories leaves
  the processes alive but breaks later loads from those directories and
  re-exec. `gc` now skips those entries and names them on stderr.

## [0.8.0] - 2026-09-20

### Added
- The tool store now bounds itself. Installing a new python-tool environment —
  through the exec path, `fetch`, or `resolve` — deletes that tool's older
  environments beyond the newest three, and logs the versions it reclaimed at
  info level. A store left to `gc` alone was never swept, because nothing ran
  `gc`: 49 versions of one tool held 14 GB here.
- Both the automatic reclaim and `binrun -- gc` now refuse to delete an
  environment a live process is running out of. A uv-installed worker execs the
  interpreter uv manages outside the environment, so the environment appears
  only in the process's argv — read through `KERN_PROCARGS2` on macOS and
  `/proc/<pid>/cmdline` on Linux, both restricted to this user's processes.
  Deleting such an environment breaks the imports the worker has not reached
  yet. `gc` names every environment it skipped on stderr.

## [0.7.0] - 2026-09-15

### Changed
- Repinned daemonkit to v0.31.0 (from v0.28.0). A `signed-app` descriptor can
  now set `app.copy_exec`, and binrun passes it through untouched. `resolve`
  and the transparent exec path then hand back a cached copy of the attested
  entrypoint rather than the file inside the app bundle. A short-lived client
  therefore never holds a live process on a bundle the next upgrade must
  quiesce. daemonkit copies only that one file, so the entrypoint must carry
  its own signature, and it keys the copy on the app's version and the
  entrypoint's identity, which keeps a warm resolve down to a few stats that
  read nothing. No command or flag changed.

## [0.6.1] - 2026-09-14

### Fixed
- binrun no longer stalls before `main` when its working directory has been
  removed. Go's package `os` calls `getcwd` at init on macOS, and libc's fallback
  scans the former parent entry by entry; a hook launched from a `$TMPDIR`
  scratch directory removed while it was starting spent minutes there. A
  `cwdguard` package that imports only `syscall`, so it initializes ahead of
  `os`, moves a removed working directory to `/` and returns to it right before
  the exec, so the artifact inherits the working directory and `$PWD` binrun was
  given.

## [0.6.0] - 2026-09-14

### Added
- A `signed-app` descriptor with a host-authoritative version can set
  `app.min_version`. An installed app older than that release fails with
  `signed app "<name>" is version X, want at least Y; run: brew upgrade
  <formula>` and exit 1; a dev build always passes. Validation refuses a
  `min_version` that is not a release triple or that sits beside a static
  version.

### Changed
- Repinned daemonkit to v0.28.0 (from v0.27.1). No command or flag changed.

## [0.5.1] - 2026-09-14

### Fixed
- A `python-tool` install refreshes uv's cached index entry for its dist.
  Right after a release, uv resolved the pinned version against a cached
  index that predated it and failed with "no version of <dist>==<version>".
  capt-hook's hooks went down that way on 12.28.0 until uv was rerun by hand
  with `--refresh-package`. Repinned daemonkit to v0.27.1 (from v0.27.0),
  which passes that flag on every install; the rest of v0.27.1 is shutdown
  handling binrun does not build against.

## [0.5.0] - 2026-09-14

### Added
- A `signed-app` descriptor can name a Homebrew formula as its upgrade hint:
  `app.formula` sits beside `app.cask`, and exactly one of the two must be set.
  A missing or stale app then prints `brew upgrade <formula>` instead of
  `brew upgrade --cask <cask>`, for apps that ship inside a formula, such as
  Captain Hook.

### Changed
- Repinned daemonkit to v0.27.0 (from v0.24.0), which carries the formula hint
  and also expands a `~/` in a signed app's `app.dir` through the passwd home.
  No command or flag changed.

## [0.4.0] - 2026-09-02

### Changed
- Repinned daemonkit to v0.24.0 (from v0.23.0), which adds the `version.file`
  descriptor source. `gc` now prunes the `python-tool` store under the same
  `--keep` it applies to the content cache.

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

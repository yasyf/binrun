# ![binrun](docs/assets/readme-banner.webp)

**Fetch, verify, and exec the exact artifact a descriptor pins.** A `.binrun` descriptor is a small executable file that names one release — version, per-platform sha256s, provider — and binrun materializes it into a content-addressed cache and process-replaces into it, so a tool can never drift from the version its plugin shipped against.

[![Release](https://img.shields.io/github/v/release/yasyf/binrun?sort=semver)](https://github.com/yasyf/binrun/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/yasyf/binrun/ci.yml?branch=main&label=ci)](https://github.com/yasyf/binrun/actions/workflows/ci.yml)
[![License: PolyForm-Noncommercial-1.0.0](https://img.shields.io/badge/License-PolyForm--Noncommercial--1.0.0-blue.svg)](https://github.com/yasyf/binrun/blob/main/LICENSE)

## Get started

```bash
brew install yasyf/tap/binrun
curl -fsSO https://raw.githubusercontent.com/yasyf/binrun/main/docs/examples/cc-review.binrun
binrun cc-review.binrun --version
```

<details>
<summary>Without Homebrew</summary>

```bash
go install github.com/yasyf/binrun/cmd/binrun@latest
```

</details>

```console
$ binrun cc-review.binrun --version
v0.25.0
```

The first run downloads the pinned release asset, verifies its sha256 and size, and unpacks it into the cache; every run after that execs straight from disk. The `--version` went to cc-review, not to binrun. After the descriptor argument, binrun interprets nothing.

Driving with an agent? Paste this:

```text
Install binrun (`brew install yasyf/tap/binrun`), write a `.binrun` descriptor
pinning my project's current release (version, per-platform asset names,
sha256s, sizes), and change the plugin's bin/ wrapper to
`exec binrun <descriptor> "$@"`.
```

---

## Use cases

### Run the binary your plugin shipped against

A plugin wrapper that installs "the latest release" breaks the moment the registry moves ahead of everything else on the machine, and the tool and its host then disagree on versions until a human intervenes. Pin the release in a descriptor committed next to the wrapper instead:

```sh
#!/bin/sh
exec binrun "$(dirname "$0")/../descriptor/cc-review.binrun" "$@"
```

The wrapper always runs the version the plugin was released with. Upgrades happen by shipping a new descriptor, not by whatever `releases/latest` says at invocation time.

### Warm the cache before you need it

`fetch` materializes without executing, and `resolve` prints where the artifact landed — both are for wiring into session-start hooks and build scripts:

```console
$ binrun -- fetch cc-review.binrun
$ binrun -- resolve cc-review.binrun
/Users/USER/.daemonkit/cache/94/941e2a99a0044153bcd49fcf1174e51a91e792b7312119363f41ccfc2304e78e/cc-review
```

The cache is content-addressed by the verified digest, so a corrupted or truncated download can never occupy the final path, and a second `fetch` of the same digest is a no-op that works offline.

### Ship a descriptor with every release

binrun's own release workflow renders its descriptor from goreleaser's output — `scripts/render-descriptor.sh` fills the version, asset names, sizes, and sha256s from `dist/` plus `checksums.txt`, validates it with `binrun -- parse`, and uploads it as a release asset. Copy that job into any repo that releases binaries and its consumers get a descriptor per release for free.

### Keep the cache bounded

```bash
binrun -- gc --keep 2
```

keeps the newest two cached versions of each artifact and removes the rest. Damaged cache entries (missing metadata) are always reclaimed.

---

## Descriptors

A descriptor is `#!/usr/bin/env binrun` followed by JSON — executable, diffable, and pinned. The committed example at [`docs/examples/cc-review.binrun`](docs/examples/cc-review.binrun) is real and runnable:

```json
{
  "schema": 1,
  "name": "cc-review",
  "kind": "release-binary",
  "version": {"static": "0.25.0"},
  "platforms": {
    "macos-aarch64": {
      "size": 11096686,
      "hash": "sha256",
      "digest": "941e2a99a0044153bcd49fcf1174e51a91e792b7312119363f41ccfc2304e78e",
      "format": "tar.gz",
      "path": "cc-review",
      "providers": [
        {"type": "github-release", "repo": "yasyf/cc-review", "tag": "v0.25.0", "name": "cc-review_0.25.0_darwin_arm64.tar.gz"}
      ]
    }
  }
}
```

| Field | Meaning |
|---|---|
| `schema` | Descriptor schema version. A runner that doesn't know the schema exits 1 loudly instead of guessing. |
| `name` | Artifact name; groups cache entries for `gc`. |
| `kind` | `release-binary`, `python-tool`, or `signed-app` (see below). |
| `version` | `{"static": "X.Y.Z"}` baked at render time, or `{"command": [...], "json_field": "..."}` resolved by asking a local host binary (see below). |
| `platforms` | One entry per platform key (`macos-aarch64`, `macos-x86_64`, `linux-x86_64`, `linux-aarch64`): asset `size`, `hash`/`digest`, archive `format` (`raw`, `tar.gz`, `zip`), the `path` of the executable inside the archive, and `providers` to fetch from. |

### Kinds

| Kind | Backend | Integrity |
|---|---|---|
| `release-binary` | GitHub release asset unpacked into the content-addressed cache at `~/.daemonkit/cache/<aa>/<digest>/<path>` | Descriptor-pinned sha256 + size, verified before anything reaches the final path |
| `python-tool` | `uv tool install <dist>==<version>` into a per-version env under `~/.daemonkit/tools`; execs the env's real entrypoint | uv's registry hash checking; offline-deterministic after first run |
| `signed-app` | Attests an installed signed app matches the pinned version; a mismatch prints the exact upgrade command | Code signature + version attestation |

### Dynamic versions

A `version.command` descriptor asks a locally installed binary what version to run — the pattern for a Python tool that must match a signed host app build-for-build. Dynamic versions are only valid for `python-tool` and `signed-app`, where an independent integrity gate (registry hashes, code signing) backs the artifact; a dynamic `release-binary` has no such gate and fails validation.

## CLI

`binrun FILE [args…]` resolves the descriptor and execs the artifact with `args` — process replacement, so the artifact's exit code, signals, and streams are binrun's. Management verbs live behind `--`:

| Verb | Does |
|---|---|
| `binrun -- fetch FILE` | Materialize without executing (pre-warm) |
| `binrun -- resolve FILE` | Print the resolved local executable path |
| `binrun -- parse FILE` | Print the normalized descriptor JSON |
| `binrun -- latest FILE` | Print the newest release tag from the descriptor's provider |
| `binrun -- gc [--keep N]` | Prune the cache to the newest N versions per artifact |
| `binrun -- cache-dir` | Print the content cache directory |

Exit codes: `0` on success; every binrun-domain failure exits `1` with a one-line message. When the artifact runs, its own exit code passes through untouched. binrun never exits `2` — in the ecosystem it serves, `2` is a hook's blocking verdict and belongs to the artifact alone.

Status: pre-1.0. Descriptor schema 1 is stable; a schema bump makes an old runner fail loudly rather than guess.

Licensed under [PolyForm-Noncommercial-1.0.0](LICENSE).

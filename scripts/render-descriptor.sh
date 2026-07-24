#!/usr/bin/env bash
# Render descriptor/binrun.binrun.tmpl into a concrete descriptor, filling the
# version and per-platform size/digest/asset-name from a goreleaser dist tree.
#
# Usage: scripts/render-descriptor.sh <dist-dir> <version>   (writes to stdout)
#
#   <dist-dir>  a directory holding the release tar.gz archives and checksums.txt
#               (goreleaser's dist/, or the assets downloaded from a release)
#   <version>   the bare release version, e.g. 0.1.0 (no leading "v")
set -euo pipefail

dist="${1:?usage: render-descriptor.sh <dist-dir> <version>}"
version="${2:?usage: render-descriptor.sh <dist-dir> <version>}"

here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmpl="$here/descriptor/binrun.binrun.tmpl"
checksums="$dist/checksums.txt"

# dotslash platform key -> goreleaser {{.Os}}_{{.Arch}}
platforms=(
  "MACOS_AARCH64:darwin_arm64"
  "MACOS_X86_64:darwin_amd64"
  "LINUX_X86_64:linux_amd64"
  "LINUX_AARCH64:linux_arm64"
)

file_size() { stat -f%z "$1" 2>/dev/null || stat -c%s "$1"; }

out="$(cat "$tmpl")"
out="${out//__VERSION__/$version}"

for pair in "${platforms[@]}"; do
  key="${pair%%:*}"
  osarch="${pair##*:}"
  name="binrun_${version}_${osarch}.tar.gz"
  digest="$(awk -v n="$name" '$2 == n {print $1}' "$checksums")"
  [[ -n "$digest" ]] || { echo "render-descriptor: no checksum for $name in $checksums" >&2; exit 1; }
  size="$(file_size "$dist/$name")"
  out="${out//__NAME_${key}__/$name}"
  out="${out//__DIGEST_${key}__/$digest}"
  out="${out//__SIZE_${key}__/$size}"
done

printf '%s\n' "$out"

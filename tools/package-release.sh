#!/bin/sh
# SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
set -eu

version="${1:-}"
output="${2:-dist}"
targets="${PAPER_RELEASE_TARGETS:-linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64}"

case "$version" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "usage: tools/package-release.sh vMAJOR.MINOR.PATCH[-PRERELEASE] [OUTPUT]" >&2; exit 2 ;;
esac

mkdir -p "$output"
go_version="$(go version | awk '{print $3}')"

for target in $targets; do
  target_os="${target%/*}"
  target_arch="${target#*/}"
  base="paperrune_${version#v}_${target_os}_${target_arch}"
  stage="$output/$base"
  executable_suffix=""
  if [ "$target_os" = "windows" ]; then executable_suffix=".exe"; fi

  mkdir -p "$stage"
  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build -trimpath -ldflags="-s -w -X main.version=$version" -o "$stage/paper$executable_suffix" ./cmd/paper
  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build -trimpath -ldflags="-s -w -X main.version=$version" -o "$stage/paper-studio$executable_suffix" ./cmd/paper-studio
  go run ./cmd/release-sbom -binary "$stage/paper$executable_suffix" -output "$stage/paper.cdx.json" -name paper -version "$version"
  go run ./cmd/release-sbom -binary "$stage/paper-studio$executable_suffix" -output "$stage/paper-studio.cdx.json" -name paper-studio -version "$version"
  cp LICENSE README.md "$stage/"
  printf 'PaperRune %s\nBuilt with %s for %s/%s\n' "$version" "$go_version" "$target_os" "$target_arch" > "$stage/BUILD.txt"

  cp "$stage/paper.cdx.json" "$output/$base-paper.cdx.json"
  cp "$stage/paper-studio.cdx.json" "$output/$base-paper-studio.cdx.json"
  if [ "$target_os" = "windows" ]; then
    (cd "$output" && zip -q -r "$base.zip" "$base")
  else
    tar -C "$output" -czf "$output/$base.tar.gz" "$base"
  fi
  rm -rf "$stage"
done

(cd "$output" && sha256sum paperrune_* > checksums.txt)

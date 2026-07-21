#!/bin/sh
set -eu

command -v pdfinfo >/dev/null 2>&1 || {
	echo "pdfinfo is required for external PDF reader smoke checks" >&2
	exit 1
}

smoke_dir=$(mktemp -d)
trap 'rm -rf "$smoke_dir"' EXIT HUP INT TERM

go run ./cmd/compliance-fixtures -out "$smoke_dir"

for pdf in "$smoke_dir"/*.pdf; do
	pdfinfo "$pdf" >/dev/null
done

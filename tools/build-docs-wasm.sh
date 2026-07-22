#!/bin/sh
# SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
# Copyright (c) 2026 cssBruno
set -eu

workspace_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
public_dir="$workspace_dir/docs/public"
wasm_runtime=$(go env GOROOT)/lib/wasm/wasm_exec.js
docs_go_cache=${PAPERRUNE_DOCS_GOCACHE:-"$workspace_dir/.gocache"}

mkdir -p "$public_dir"
cd "$workspace_dir"
GOCACHE="$docs_go_cache" GOOS=js GOARCH=wasm go build -trimpath -buildvcs=false \
	-ldflags='-s -w -buildid=' \
	-o "$public_dir/paperrune.wasm" ./cmd/paper-studio-wasm
cp "$wasm_runtime" "$public_dir/wasm_exec.js"
chmod 0644 "$public_dir/paperrune.wasm" "$public_dir/wasm_exec.js"

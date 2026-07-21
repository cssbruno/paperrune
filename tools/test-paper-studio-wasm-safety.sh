#!/bin/sh
# SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
set -eu

workspace_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
temporary_dir=$(mktemp -d "${TMPDIR:-/tmp}/paper-studio-wasm-safety.XXXXXX")
server_binary="$temporary_dir/paper-studio"
server_log="$temporary_dir/server.log"
server_pid=""
port=${PAPER_STUDIO_WASM_SAFETY_PORT:-17333}

cleanup_server() {
	if [ -n "$server_pid" ]; then
		kill "$server_pid" 2>/dev/null || true
		wait "$server_pid" 2>/dev/null || true
		server_pid=""
	fi
}
cleanup() {
	cleanup_server
	rm -rf "$temporary_dir"
}
trap cleanup EXIT INT TERM

cd "$workspace_dir"
make paper-studio-wasm-go \
	PAPER_STUDIO_WASM="$temporary_dir/go.wasm" \
	PAPER_STUDIO_WASM_GZIP="$temporary_dir/go.wasm.gz" \
	PAPER_STUDIO_WASM_BROTLI="$temporary_dir/go.wasm.br" \
	PAPER_STUDIO_WASM_EXEC="$temporary_dir/go-wasm_exec.js"
make paper-studio-wasm-tinygo \
	PAPER_STUDIO_WASM="$temporary_dir/tinygo.wasm" \
	PAPER_STUDIO_WASM_GZIP="$temporary_dir/tinygo.wasm.gz" \
	PAPER_STUDIO_WASM_BROTLI="$temporary_dir/tinygo.wasm.br" \
	PAPER_STUDIO_WASM_EXEC="$temporary_dir/tinygo-wasm_exec.js"
go build -o "$server_binary" ./cmd/paper-studio

for fixture in "$workspace_dir"/testdata/paper/*.paper; do
	if [ -n "${PAPER_STUDIO_WASM_FIXTURE:-}" ] && [ "$(basename "$fixture")" != "$PAPER_STUDIO_WASM_FIXTURE" ]; then
		continue
	fi
	"$server_binary" -addr "127.0.0.1:$port" \
		-assets "$workspace_dir/testdata/paper/studio-assets.json" \
		-asset-root "$workspace_dir/testdata/paper/assets" \
		"$fixture" >"$server_log" 2>&1 &
	server_pid=$!
	attempt=0
	session_token=""
	until [ -n "$session_token" ] && curl -fsS -H "X-Paper-Studio-Token: $session_token" "http://127.0.0.1:$port/api/workspace" >/dev/null 2>&1; do
		session_token=$(sed -n 's/.*#token=//p' "$server_log")
		attempt=$((attempt + 1))
		if [ "$attempt" -ge 200 ]; then
			cat "$server_log"
			exit 1
		fi
		sleep 0.05
	done

	fuzz_cases=0
	soak_renders=0
	if [ "$(basename "$fixture")" = "studio-demo.paper" ]; then
		fuzz_cases=${PAPER_STUDIO_WASM_FUZZ_CASES:-64}
		soak_renders=${PAPER_STUDIO_WASM_SOAK_RENDERS:-100}
	fi
	PAPER_STUDIO_WASM_FUZZ_CASES="$fuzz_cases" \
	PAPER_STUDIO_WASM_SOAK_RENDERS="$soak_renders" \
		node tools/test-paper-studio-wasm-safety.mjs \
		"http://127.0.0.1:$port" "$session_token" \
		"$temporary_dir/go.wasm" "$temporary_dir/go-wasm_exec.js" \
		"$temporary_dir/tinygo.wasm" "$temporary_dir/tinygo-wasm_exec.js"
	cleanup_server
done

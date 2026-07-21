#!/bin/sh
# SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
# Copyright (c) 2026 cssBruno

set -eu

root=$(git rev-parse --show-toplevel)
revision=$(git -C "$root" rev-parse HEAD)
temporary=$(mktemp -d "${TMPDIR:-/tmp}/paperrune-clean-checkout.XXXXXX")
archive="$temporary/source.tar"
checkout="$temporary/checkout"
trap 'rm -rf "$temporary"' EXIT HUP INT TERM

mkdir -p "$checkout"
git -C "$root" archive --format=tar --output="$archive" "$revision"
tar -xf "$archive" -C "$checkout"

cache="$temporary/go-cache"
cd "$checkout"
GOCACHE="$cache" go test ./document -run '^(TestTypedCharacterizationCorpusIsCompleteBoundedAndDeterministic|TestHTMLCharacterizationFixturesExerciseEveryClassification)$' -count=1

GOCACHE="$cache" go test ./document ./internal/layoutengine -run '^$' \
  -bench '^BenchmarkPaperEngine(Planner|Painter|ProductionDefault|EndToEnd|WarmCompiled|Concurrent|Table)' \
  -benchmem -benchtime="${CLEAN_CHECKOUT_BENCHTIME:-250ms}" -count="${CLEAN_CHECKOUT_BENCH_COUNT:-10}" >/dev/null

printf 'clean checkout verified at %s\n' "$revision"

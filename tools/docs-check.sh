#!/bin/sh
# SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
set -eu

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT HUP INT TERM

go build -o "$work/paper" ./cmd/paper
"$work/paper" help >/dev/null
"$work/paper" help render >/dev/null
"$work/paper" check --json --data examples/invoice/example.json examples/invoice/invoice.paper >/dev/null
"$work/paper" render --data examples/invoice/example.json -o "$work/invoice.pdf" examples/invoice/invoice.paper >/dev/null
test -s "$work/invoice.pdf"

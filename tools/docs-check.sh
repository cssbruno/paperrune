#!/bin/sh
# SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
set -eu

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT HUP INT TERM

go build -o "$work/paper" ./cmd/paper
"$work/paper" help >/dev/null
"$work/paper" help render >/dev/null
"$work/paper" init invoice "$work/my-invoice" >/dev/null
(
  cd "$work/my-invoice"
  "$work/paper" check --json >/dev/null
  "$work/paper" render >/dev/null
  test -s dist/invoice.pdf
)

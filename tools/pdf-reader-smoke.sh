#!/bin/sh
set -eu

command -v pdfinfo >/dev/null 2>&1 || {
	echo "pdfinfo is required for external PDF reader smoke checks" >&2
	exit 1
}

found=0
for pdf in assets/generated/pdf/*.pdf; do
	[ -f "$pdf" ] || continue
	found=1
	pdfinfo "$pdf" >/dev/null
done

if [ "$found" -eq 0 ]; then
	echo "no tracked PDF fixtures found" >&2
	exit 1
fi

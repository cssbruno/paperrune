#!/bin/sh
# SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
set -eu

destination="${1:-}"
if [ -z "$destination" ]; then
	echo "usage: tools/install-wasm-toolchain.sh DESTINATION" >&2
	exit 2
fi

tinygo_version=0.41.1
binaryen_version=131
system=$(uname -s)
machine=$(uname -m)

case "$system/$machine" in
	Linux/x86_64)
		tinygo_platform=linux-amd64
		tinygo_sha256=e156d1d93a376eef639a4143d13be07e8c463fb6cf2d7d447698ed4474d23e91
		binaryen_platform=x86_64-linux
		binaryen_sha256=b5bf1f0eaf17c63ee588ff7a5954dc8f6ce2c26989051c66f24dfe9ece3e46db
		;;
	Linux/aarch64|Linux/arm64)
		tinygo_platform=linux-arm64
		tinygo_sha256=789733bc3b5bace0bd1835a267b3ea267804a7ef1cfe69bc522c295f5226d624
		binaryen_platform=aarch64-linux
		binaryen_sha256=ba991f677edd9a21d2bc96c0144bc8ac5b112d4d98a3eb266e075e22e557df2a
		;;
	Darwin/arm64)
		tinygo_platform=darwin-arm64
		tinygo_sha256=c684d154d89a452cc9c7fc5dc5fc80cb6a42445b3e44b3c12ed048692de0f341
		binaryen_platform=arm64-macos
		binaryen_sha256=e441b48dc22163d209b4f05e44dc7210909b01237642b6c9ae48fd710a3ef83e
		;;
	*)
		echo "unsupported WASM toolchain host: $system/$machine" >&2
		exit 1
		;;
esac

temporary_dir=$(mktemp -d "${TMPDIR:-/tmp}/paperrune-wasm-tools.XXXXXX")
cleanup() {
	rm -rf "$temporary_dir"
}
trap cleanup EXIT INT TERM

tinygo_archive="tinygo${tinygo_version}.${tinygo_platform}.tar.gz"
binaryen_archive="binaryen-version_${binaryen_version}-${binaryen_platform}.tar.gz"
curl --proto '=https' --tlsv1.2 -fsSL \
	-o "$temporary_dir/$tinygo_archive" \
	"https://github.com/tinygo-org/tinygo/releases/download/v${tinygo_version}/${tinygo_archive}"
curl --proto '=https' --tlsv1.2 -fsSL \
	-o "$temporary_dir/$binaryen_archive" \
	"https://github.com/WebAssembly/binaryen/releases/download/version_${binaryen_version}/${binaryen_archive}"

actual_tinygo_sha256=$(shasum -a 256 "$temporary_dir/$tinygo_archive" | awk '{print $1}')
actual_binaryen_sha256=$(shasum -a 256 "$temporary_dir/$binaryen_archive" | awk '{print $1}')
if [ "$actual_tinygo_sha256" != "$tinygo_sha256" ]; then
	echo "TinyGo checksum mismatch" >&2
	exit 1
fi
if [ "$actual_binaryen_sha256" != "$binaryen_sha256" ]; then
	echo "Binaryen checksum mismatch" >&2
	exit 1
fi

mkdir -p "$destination/bin" "$destination/tinygo" "$destination/binaryen"
tar -xzf "$temporary_dir/$tinygo_archive" -C "$destination/tinygo" --strip-components=1
tar -xzf "$temporary_dir/$binaryen_archive" -C "$destination/binaryen" --strip-components=1
ln -sf "$destination/tinygo/bin/tinygo" "$destination/bin/tinygo"
ln -sf "$destination/binaryen/bin/wasm-opt" "$destination/bin/wasm-opt"

"$destination/bin/tinygo" version
"$destination/bin/wasm-opt" --version
printf '%s\n' "$destination/bin"

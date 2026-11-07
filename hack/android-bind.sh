#!/usr/bin/env bash
# Bind the on-device Go core into an Android archive.
#
# gomobile's JNI wrapper is cgo. The Daywire packages in that archive are the
# pure-Go store, crypt, and event code. This script does not pass -tags speaker
# and does not link sherpa or onnxruntime. Week 3 speech inference is the
# separate sherpa-onnx Android archive from hack/fetch-speech-models.sh.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

if [[ -z "${ANDROID_HOME:-}" && -z "${ANDROID_SDK_ROOT:-}" ]]; then
	echo "Set ANDROID_HOME to the Android SDK, or run ./hack/make.sh apk to install the command-line SDK and build the debug APK." >&2
	exit 1
fi
export ANDROID_HOME="${ANDROID_HOME:-${ANDROID_SDK_ROOT}}"

if ! command -v unzip >/dev/null 2>&1; then
	echo "unzip is required to check the archive for speech libraries" >&2
	exit 1
fi

# Drop a partial publish left by a killed earlier run. The trap covers this run.
rm -f android/app/libs/daywire.aar.partial

gopath=$(go env GOPATH)
# Install the module's gomobile and gobind into a local bin so a stale
# copy earlier on PATH cannot win. Versions come from go.mod.
bindbin="${root}/output/android-bind-bin"
mkdir -p "${bindbin}"
GOBIN="${bindbin}" go install golang.org/x/mobile/cmd/gomobile golang.org/x/mobile/cmd/gobind
export PATH="${bindbin}:${gopath}/bin:${PATH}"

if [[ ! -d "${ANDROID_HOME}/ndk" && ! -d "${ANDROID_HOME}/ndk-bundle" && -z "${ANDROID_NDK_HOME:-}" ]]; then
	echo "No Android NDK found under ANDROID_HOME. Install NDK 28.2.13676358 (side by side), or run ./hack/make.sh apk." >&2
	exit 1
fi

# gomobile writes a patchless `go 1.26` line into its temp module. With
# GOTOOLCHAIN=auto the child `go` then looks for a toolchain named go1.26,
# which is not a release. Run the Go 1.26 binary already selected for this
# module and keep it local.
toolchain_root=$(go env GOROOT)
realgo="${toolchain_root}/bin/go"
if [[ ! -x "${realgo}" ]]; then
	echo "Go toolchain not found at ${realgo}. Need Go 1.26 (GOTOOLCHAIN=auto)." >&2
	exit 1
fi
# Portable on GNU and BSD/macOS. The directory also holds the sibling
# sources jar gomobile writes next to the staged archive.
out="android/app/libs/daywire.aar"
partial="${out}.partial"
rm -f "${partial}"
wrap=$(mktemp -d)
trap 'rm -rf "${wrap}"; rm -f "${partial}"' EXIT
# Single-quote a string for the generated wrapper. A ' becomes '\''.
shell_quote() {
	local escaped
	escaped=$(printf '%s' "$1" | sed "s/'/'\\\\''/g")
	printf "'%s'" "${escaped}"
}
{
	printf '%s\n' '#!/bin/sh'
	printf '%s\n' 'export GOTOOLCHAIN=local'
	printf 'export GOROOT=%s\n' "$(shell_quote "${toolchain_root}")"
	printf 'export PATH=%s\n' "$(shell_quote "${toolchain_root}/bin:${PATH}")"
	printf 'exec %s "$@"\n' "$(shell_quote "${realgo}")"
} > "${wrap}/go"
chmod +x "${wrap}/go"
export PATH="${wrap}:${PATH}"

# modernc's musl port issues legacy x86_64 syscalls (lstat, stat, open).
# Android's app seccomp filter kills those with SIGSYS, so the emulator
# crashes while opening the store. The Go overlay flag refuses to replace
# a file inside the module cache, and a workspace replace hides gomobile's
# temporary module, so this bind patches the extracted source for the
# duration of the bind and restores it on the way out. The laptop binary
# does not see the patch. The pinned hash is syscall_musl.go from
# modernc.org/libc v1.79.0.
# A fresh module cache has the sum but not the unpacked tree, so
# `go list -m -f {{.Dir}}` is empty until this download.
go mod download modernc.org/libc
libc_dir=$(go list -m -f '{{.Dir}}' modernc.org/libc)
if [[ -z "${libc_dir}" || ! -f "${libc_dir}/syscall_musl.go" ]]; then
	echo "modernc.org/libc source was not unpacked" >&2
	exit 1
fi
libc_sys="${libc_dir}/syscall_musl.go"
libc_want="20599ae6a0036b36e28b6794aa32321ffea6aa2fd88775d1b7d7ff32afd25b07"
libc_got=$(sha256sum "${libc_sys}" | awk '{print $1}')
if [[ "${libc_got}" != "${libc_want}" ]]; then
	echo "modernc.org/libc syscall_musl.go changed. Update hack/android-libc/syscall_musl.go.overlay before binding." >&2
	exit 1
fi
libc_backup="${wrap}/syscall_musl.go.orig"
cp -a "${libc_sys}" "${libc_backup}"
chmod u+w "${libc_sys}"
cp -f "${root}/hack/android-libc/syscall_musl.go.overlay" "${libc_sys}"
trap 'cp -a "${libc_backup}" "${libc_sys}"; rm -rf "${wrap}"; rm -f "${partial}"' EXIT

gomobile init

staged="${wrap}/daywire.aar"
# Phones are arm64. The Studio emulator on a typical laptop is amd64.
# Add android/arm,android/386 to -target if a 32-bit device is required.
gomobile bind \
	-target=android/arm64,android/amd64 \
	-androidapi 29 \
	-javapkg=com.chenbenja.daywire.core \
	-o "${staged}" \
	github.com/chenbenja/daywire/mobile

listing=$(unzip -l "${staged}") || {
	echo "could not list the bind output" >&2
	exit 1
}
if printf '%s\n' "${listing}" | grep -Ei 'sherpa|onnxruntime|libonnx'; then
	echo "bind output contains a speech library" >&2
	exit 1
fi

mkdir -p "$(dirname "${out}")"
# Copy onto the destination filesystem, then rename into place so an
# interrupted cross-directory move cannot leave a partial daywire.aar.
partial="${out}.partial"
rm -f "${partial}"
cp -f "${staged}" "${partial}"
mv -f "${partial}" "${out}"
echo "wrote ${out}"

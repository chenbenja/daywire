#!/usr/bin/env bash
# Bind the on-device Go core into an Android archive.
#
# gomobile's JNI wrapper is cgo. The Daywire packages in that archive are the
# pure-Go store, crypt, and event code. This script does not pass -tags speaker
# and does not link sherpa or onnxruntime.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

if [[ -z "${ANDROID_HOME:-}" && -z "${ANDROID_SDK_ROOT:-}" ]]; then
	echo "Set ANDROID_HOME to the Android SDK (Android Studio: Settings, Languages & Frameworks, Android SDK)." >&2
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
	echo "No Android NDK found under ANDROID_HOME. In Android Studio's SDK manager, install NDK (Side by side). The current plugin default is 28.2.13676358." >&2
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

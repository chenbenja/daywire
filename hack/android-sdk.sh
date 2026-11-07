#!/usr/bin/env bash
# Install the Android command-line SDK pieces the debug APK uses.
#
# Android Studio is not required. Platform 36, build-tools 36.0.0, and NDK
# 28.2.13676358 match the Gradle project and hack/android-bind.sh. Re-running
# is a no-op once those packages are present. The Linux command-line tools
# zip is pinned; other operating systems must already have ANDROID_HOME.
set -euo pipefail

if [[ -z "${ANDROID_HOME:-}" && -z "${ANDROID_SDK_ROOT:-}" ]]; then
	export ANDROID_HOME="${HOME}/Android/Sdk"
else
	export ANDROID_HOME="${ANDROID_HOME:-${ANDROID_SDK_ROOT}}"
fi
export ANDROID_SDK_ROOT="${ANDROID_HOME}"

platform="${ANDROID_HOME}/platforms/android-36"
build_tools="${ANDROID_HOME}/build-tools/36.0.0"
ndk="${ANDROID_HOME}/ndk/28.2.13676358"

if [[ -d "${platform}" && -x "${build_tools}/aapt2" && -d "${ndk}/toolchains" ]]; then
	echo "Android SDK ready at ${ANDROID_HOME}"
	exit 0
fi

if [[ "$(uname -s)" != "Linux" ]]; then
	echo "Android SDK at ${ANDROID_HOME} is missing platform 36, build-tools 36.0.0, or NDK 28.2.13676358." >&2
	echo "This script downloads those packages on Linux. On another OS, install them and set ANDROID_HOME." >&2
	exit 1
fi

if [[ -z "${JAVA_HOME:-}" ]]; then
	java_bin=$(command -v java || true)
	if [[ -z "${java_bin}" ]]; then
		echo "JDK 17 or newer is required to install the Android SDK." >&2
		exit 1
	fi
	java_bin=$(readlink -f "${java_bin}")
	export JAVA_HOME
	JAVA_HOME=$(dirname "$(dirname "${java_bin}")")
fi

if ! command -v curl >/dev/null 2>&1 || ! command -v unzip >/dev/null 2>&1; then
	echo "curl and unzip are required to install the Android SDK." >&2
	exit 1
fi

cmdline="${ANDROID_HOME}/cmdline-tools/latest/bin/sdkmanager"
if [[ ! -x "${cmdline}" ]]; then
	url="https://dl.google.com/android/repository/commandlinetools-linux-16111833_latest.zip"
	sha="0877a1d048fe4a24efe2eff536ca4223f7adeb58648bb81909d33c446918cfa8"
	tmp=$(mktemp -d)
	trap 'rm -rf "${tmp}"' EXIT
	echo "fetch Android command-line tools"
	curl -fL --retry 3 --retry-delay 2 -o "${tmp}/cmdtools.zip" "${url}"
	got=$(sha256sum "${tmp}/cmdtools.zip" | awk '{print $1}')
	if [[ "${got}" != "${sha}" ]]; then
		echo "checksum mismatch for Android command-line tools" >&2
		exit 1
	fi
	unzip -q "${tmp}/cmdtools.zip" -d "${tmp}"
	mkdir -p "${ANDROID_HOME}/cmdline-tools"
	rm -rf "${ANDROID_HOME}/cmdline-tools/latest"
	mv "${tmp}/cmdline-tools" "${ANDROID_HOME}/cmdline-tools/latest"
	rm -rf "${tmp}"
	trap - EXIT
fi

mkdir -p "${ANDROID_HOME}/licenses"
printf '%s\n' "24333f8a63b6825ea9c5514f83c2829b004d1fee" > "${ANDROID_HOME}/licenses/android-sdk-license"

echo "install Android SDK platform 36, build-tools 36.0.0, NDK 28.2.13676358"
"${cmdline}" --sdk_root="${ANDROID_HOME}" --install \
	"platforms;android-36" \
	"build-tools;36.0.0" \
	"ndk;28.2.13676358" \
	"platform-tools"

if [[ ! -d "${platform}" || ! -x "${build_tools}/aapt2" || ! -d "${ndk}/toolchains" ]]; then
	echo "Android SDK install did not produce platform 36, build-tools 36.0.0, and NDK 28.2.13676358 under ${ANDROID_HOME}" >&2
	exit 1
fi
echo "Android SDK ready at ${ANDROID_HOME}"

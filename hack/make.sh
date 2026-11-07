#!/usr/bin/env bash
# Build, format-check, vet, and test the Daywire module.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

files() {
	find . -name '*.go' -not -path './output/*' -print0
}

fmt_check() {
	local unformatted
	unformatted=$(files | xargs -0 gofmt -l)
	if [[ -n "${unformatted}" ]]; then
		echo "gofmt needed:"
		echo "${unformatted}"
		exit 1
	fi
}

fmt_write() {
	files | xargs -0 gofmt -w
}

ensure_android_home() {
	if [[ -z "${ANDROID_HOME:-}" && -z "${ANDROID_SDK_ROOT:-}" ]]; then
		export ANDROID_HOME="${HOME}/Android/Sdk"
	else
		export ANDROID_HOME="${ANDROID_HOME:-${ANDROID_SDK_ROOT}}"
	fi
	export ANDROID_SDK_ROOT="${ANDROID_HOME}"
	export ANDROID_NDK_HOME="${ANDROID_HOME}/ndk/28.2.13676358"
	if [[ -z "${JAVA_HOME:-}" ]]; then
		local java_bin
		java_bin=$(command -v java || true)
		if [[ -n "${java_bin}" ]]; then
			java_bin=$(readlink -f "${java_bin}")
			JAVA_HOME=$(dirname "$(dirname "${java_bin}")")
			export JAVA_HOME
		fi
	fi
}

write_local_properties() {
	local props escaped
	props="${root}/android/local.properties"
	escaped=$(printf '%s' "${ANDROID_HOME}" | sed -e 's/\\/\\\\/g' -e 's/:/\\:/g' -e 's/ /\\ /g')
	printf 'sdk.dir=%s\n' "${escaped}" > "${props}"
}

case "${1:-all}" in
fmt)
	fmt_check
	;;
fmt-write)
	fmt_write
	;;
vet)
	go vet ./...
	;;
test)
	go test -count=1 -race ./...
	;;
build)
	mkdir -p output
	CGO_ENABLED=0 go build -o output/daywire ./cmd/daywire
	;;
speaker)
	fmt_check
	go vet -tags speaker ./...
	go test -count=1 -race -tags speaker ./internal/speaker/ ./internal/cli/
	mkdir -p output
	go build -tags speaker -o output/daywire-speaker ./cmd/daywire
	;;
all)
	fmt_check
	go vet ./...
	go test -count=1 -race ./...
	mkdir -p output
	CGO_ENABLED=0 go build -o output/daywire ./cmd/daywire
	;;
android)
	# Separate from `all`: the AAR bind needs the Android SDK and gomobile.
	"$root/hack/android-bind.sh"
	;;
speech-models)
	# Separate from `all`: downloads pinned models. The phone does not.
	"$root/hack/fetch-speech-models.sh"
	;;
apk)
	# Debug APK from a checkout: command-line SDK, pinned models, Go bind,
	# unit tests, assembleDebug. Does not pass -tags speaker. `all` stays
	# the laptop binary and does not download models.
	ensure_android_home
	status=0
	"$root/hack/android-sdk.sh" &
	pid_sdk=$!
	"$root/hack/fetch-speech-models.sh" &
	pid_fetch=$!
	wait "${pid_sdk}" || status=$?
	wait "${pid_fetch}" || status=$?
	if [[ "${status}" -ne 0 ]]; then
		exit "${status}"
	fi
	"$root/hack/android-bind.sh"
	write_local_properties
	(
		cd "${root}/android"
		./gradlew --no-daemon :app:testDebugUnitTest :app:assembleDebug
	)
	echo "debug APK: ${root}/android/app/build/outputs/apk/debug/app-debug.apk"
	;;
*)
	echo "usage: $0 {fmt|fmt-write|vet|test|build|speaker|android|speech-models|apk|all}" >&2
	exit 2
	;;
esac

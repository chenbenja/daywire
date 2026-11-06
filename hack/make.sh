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
*)
	echo "usage: $0 {fmt|fmt-write|vet|test|build|speaker|android|speech-models|all}" >&2
	exit 2
	;;
esac

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
	go build -o output/daywire ./cmd/daywire
	;;
all)
	fmt_check
	go vet ./...
	go test -count=1 -race ./...
	mkdir -p output
	go build -o output/daywire ./cmd/daywire
	;;
*)
	echo "usage: $0 {fmt|fmt-write|vet|test|build|all}" >&2
	exit 2
	;;
esac

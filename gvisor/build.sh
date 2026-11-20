#!/bin/sh
# Builds runsc into ../runsc and the sidecar binaries it needs into ../gvisor-bin/.
# runsc looks for its sidecars in a gvisor-bin directory next to itself.
set -e
cd "$(dirname "$0")"
export CGO_ENABLED=0
go get gvisor.dev/gvisor/runsc@go
TMP=$(mktemp -d)
trap 'chmod -R u+w "$TMP"; rm -rf "$TMP"' EXIT
trap 'exit 1' HUP INT TERM
# The go branch ships systrap's precompiled seccomp programs for amd64 only, so
# build from a copy of the module with them regenerated for the host architecture.
SRC="$TMP/gvisor"
cp -r "$(go list -m -f '{{.Dir}}' gvisor.dev/gvisor)" "$SRC"
chmod -R u+w "$SRC"
cp go.mod go.sum "$TMP/"
go mod edit -modfile="$TMP/go.mod" -replace gvisor.dev/gvisor="$SRC"
# -trimpath keeps the random module path out of build cache keys.
export GOFLAGS="$GOFLAGS -trimpath -mod=mod -modfile=$TMP/go.mod"
go run ./precompile > "$SRC/pkg/sentry/platform/systrap/stubfilter_precompiled.go"
go build -o ../runsc gvisor.dev/gvisor/runsc
mkdir -p ../gvisor-bin
go build -o ../gvisor-bin/gvisor_sentry gvisor.dev/gvisor/runsc/cmd/sentry
# The C sidecars are freestanding static binaries; flags mirror their upstream BUILD files.
for c in prewarmer:gvisor-sentry-prewarmer procumounter:gvisor-proc-umounter fdparking:runsc-fd-parking; do
	${CC:-cc} -Os -ffreestanding -fno-sanitize=all -fno-stack-protector \
		-fno-asynchronous-unwind-tables -fno-unwind-tables -fno-pie -no-pie \
		-static -nostdlib -nostartfiles \
		-Wl,--build-id=none -Wl,--gc-sections -Wl,-s -Wl,-n \
		-o "../gvisor-bin/${c#*:}" "$SRC/runsc/${c%%:*}/${c%%:*}.c"
done

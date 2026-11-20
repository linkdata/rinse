#!/bin/sh
set -e
go generate ./... || true
go run github.com/swaggo/swag/cmd/swag@v1.16.6 fmt
go run github.com/securego/gosec/v2/cmd/gosec@v2.30.0 -exclude=G120,G703,G704,G705,G706 -exclude-dir=gvisor ./...
CGO_ENABLED=0 go build $@ .
CGO_ENABLED=0 go build -tags devel -o rinse-devel $@ .
./gvisor/build.sh
TIKAVERSION=$(curl -s https://dlcdn.apache.org/tika/ | grep -oE '3\.[0-9]+\.[0-9]+' | head -n 1)
podman build --build-arg TIKAVERSION=$TIKAVERSION -t localhost/rinse .
mkdir -p rootfs
podman unshare ./copyrootfs.sh
rm -rf rinse rinse-devel runsc gvisor-bin
RINSE_SELFTEST=1 ./run.sh
trivy image --download-db-only && trivy image --download-java-db-only && trivy image localhost/rinse

#!/bin/sh
set -e
go generate ./... || true
go run github.com/swaggo/swag/cmd/swag@latest fmt
gosec -exclude=G703,G704,G705,G706 ./...
CGO_ENABLED=0 go build $@ .
CGO_ENABLED=0 go build -tags devel -o rinse-devel $@ .
cd gvisor
GO111MODULE=on go get gvisor.dev/gvisor/runsc@go
CGO_ENABLED=0 GO111MODULE=on go build -o ../runsc gvisor.dev/gvisor/runsc
cd ..
TIKAVERSION=$(curl -s https://dlcdn.apache.org/tika/ | grep -oE '3\.[0-9]+\.[0-9]+' | head -n 1)
podman build --build-arg TIKAVERSION=$TIKAVERSION -t localhost/rinse .
mkdir -p rootfs
podman unshare ./copyrootfs.sh
rm rinse rinse-devel runsc
RINSE_SELFTEST=1 ./run.sh
trivy image --download-db-only && trivy image --download-java-db-only && trivy image localhost/rinse

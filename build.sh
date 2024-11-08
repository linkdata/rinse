#!/bin/sh
set -e
go generate ./... || true
go run github.com/swaggo/swag/cmd/swag@latest fmt
CGO_ENABLED=0 go build $@ .
CGO_ENABLED=0 go build -tags devel -o rinse-devel $@ .
cd gvisor
GO111MODULE=on go get gvisor.dev/gvisor/runsc@go
CGO_ENABLED=0 GO111MODULE=on go build -o ../runsc gvisor.dev/gvisor/runsc
cd ..
podman build -t localhost/rinse .
mkdir -p rootfs
podman unshare ./copyrootfs.sh
rm rinse rinse-devel runsc
trivy image localhost/rinse || true

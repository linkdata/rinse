#!/bin/sh
go generate ./...
CGO_ENABLED=0 go build $@ . && \
CGO_ENABLED=0 go build -tags devel -o rinse-devel $@ . && \
podman build -t localhost/rinse . && \
mkdir -p rootfs && \
podman unshare ./copyrootfs.sh && \
rm rinse rinse-devel

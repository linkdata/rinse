#!/bin/sh -e
IMAGE='localhost/rinse'
[ -n "$1" ] && IMAGE=$1
wd=$(pwd)
if [ ! -d "$wd/rootfs" ]; then
	mkdir -p $wd/rootfs
	podman unshare cp -rp $(podman image mount $IMAGE)/* $wd/rootfs/
fi
go run . -selftest

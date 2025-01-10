#!/bin/sh -e
if [ $(id -u) -ne 0 ]; then
  podman unshare $0
  exit
fi
IMAGE='localhost/rinse'
[ -n "$1" ] && IMAGE=$1
WORKDIR=$(pwd)
mkdir -p $WORKDIR/rootfs
cp -rp $(podman image mount $IMAGE)/* $WORKDIR/rootfs/
podman image unmount $IMAGE > /dev/null
echo To remove $WORKDIR/rootfs: podman unshare rm -rf $WORKDIR/rootfs/

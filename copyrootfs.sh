#!/bin/sh -e
if [ $(id -u) -ne 0 ]; then
  podman unshare $0 $*
  exit
fi

IMAGE="localhost/rinse"
if [ -n "$1" ]; then
  IMAGE=$1
fi

SRCDIR=$(podman image mount $IMAGE)
if [ -n "$SRCDIR" ]; then
  WORKDIR=$(pwd)
  mkdir -p $WORKDIR/rootfs
  cp -rp $SRCDIR/* $WORKDIR/rootfs/
  podman image unmount $IMAGE > /dev/null
  echo To remove $WORKDIR/rootfs: podman unshare rm -rf $WORKDIR/rootfs/
fi

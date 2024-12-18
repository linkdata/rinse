#!/bin/sh -e
if [ $(id -u) -ne 0 ]; then
  podman unshare $0
  exit
fi
wd=$(pwd)
mkdir -p $wd/rootfs
cp -rp $(podman image mount localhost/rinse)/* $wd/rootfs/
podman image unmount localhost/rinse > /dev/null
echo To remove rootfs: podman unshare rm -rf rootfs/

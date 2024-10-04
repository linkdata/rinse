#!/bin/sh
mnt=$(podman image mount localhost/rinse)
cp -rp $mnt/* rootfs/
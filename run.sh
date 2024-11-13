#!/bin/sh
RINSE_PORT=8080
mkdir -p /tmp/rinse && chmod 777 /tmp/rinse
podman run --rm --read-only --replace --name rinse \
	--cap-drop=ALL --cap-add=CAP_SYS_CHROOT \
	 -v /tmp/rinse:/etc/rinse \
	--env RINSE_PORT=$RINSE_PORT -p $RINSE_PORT:8080 \
	--entrypoint /usr/bin/rinse-devel -it localhost/rinse
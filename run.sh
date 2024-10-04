#!/bin/sh
RINSE_PORT=8080
podman run --rm --read-only --replace --name rinse \
	--cap-add SYS_ADMIN -v /proc:/newproc:ro \
	--env RINSE_PORT=$RINSE_PORT -p $RINSE_PORT:80 \
	--entrypoint /usr/bin/rinse-devel -it localhost/rinse
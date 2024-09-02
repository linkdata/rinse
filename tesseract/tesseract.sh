#!/bin/sh
ls /var/rinse/*.ppm | tesseract - /var/rinse/output pdf
echo RINSE_EXIT

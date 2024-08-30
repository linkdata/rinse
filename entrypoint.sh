#!/bin/sh
pdftoppm /var/rinse/input.pdf /var/rinse/output && ls /var/rinse/*.ppm | tee /var/rinse/output.txt && tesseract /var/rinse/output.txt /var/rinse/output pdf && sha1sum /var/rinse/output.pdf
echo RINSE_EXIT

#!/bin/sh
ls /var/rinse/*.ppm | tesseract -l $TESS_LANG - /var/rinse/output pdf
echo RINSE_EXIT

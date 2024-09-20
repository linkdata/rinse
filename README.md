# rinse

Web service that converts untrusted documents to image-based PDF:s in a sandbox.

## Requirements

* [podman](https://podman.io/) is required.
* [gVisor](https://gvisor.dev/) is highly recommended, but optional.
* 2GB+ of disk space on `/tmp`, as the conversion process can use up a lot of space.

## Container security

The container is based on [Alpine Linux](https://www.alpinelinux.org/) and is run
with read-only filesystem, no privileges and no network (except if downloading
the file has been requested).

If you have gVisor installed and run `rinse` as root (which gVisor requires),
gVisor will be used to further sandbox the container.

## Process

First, a temporary directory is created for the job. This will be mounted in the 
container as `/var/rinse`.

Then, each of these stages run in their own podman container, which is destroyed 
as soon as the stage is complete or fails. When the job is removed, all it's files
are overwritten before they are deleted from the filesystem.

- If we were given an URL, we use [`wget`](https://www.gnu.org/software/wget/)
  from within the container to download the document. This stage allows the
  container to access the network.

The original document is renamed to `input` with it's extension preserved and made
read-only before invoking the next stage.

- If the language is to be auto-detected, [Apache Tika](https://tika.apache.org/)
  is used to do so.

- If the document is not a PDF, [LibreOffice](https://www.libreoffice.org/) is
  used to try to covert it to one, and if successful, the original document
  is deleted.

- The `input.pdf` file is converted to a set of PNG files using
  [`pdftoppm`](https://poppler.freedesktop.org/).

- The set of PNG files is OCR-ed and processed into a PDF named
  `output.pdf` using [`tesseract`](https://tesseract-ocr.github.io/).

- Finally the `output.pdf` file is renamed to the original filename
  (without extension) with `-rinsed.pdf` appended.

# rinse

Web service that converts untrusted documents to image-based PDF:s in a sandbox.

## Requirements

* [podman](https://podman.io/) is required.
* [gVisor](https://gvisor.dev/) is highly recommended, but optional.
* 2GB+ of disk space on `/tmp`, as the conversion process can use up a lot of space.

## Container security

The container is run with read-only filesystem, no privileges and no network.
If you have gVisor installed and run `rinse` as root (which gVisor requires),
gVisor will be used to further sandbox the container.

## Process

First, a temporary directory is created for the job. This will be mounted in the 
container as `/var/rinse`.

Then, each of these stages run in their own container, which is destroyed as 
soon as the stage is complete or fails.

- If we were given an URL, we use `wget` from the container to download
the document. This stage allows the container to access the network (except
for localhost).

- The original document is renamed to `input` with it's extension preserved.

- If the language is to be auto-detected, Apache Tika is used to do so.

- If the document is not a PDF, LibreOffice is used to try to covert it to one,
and if successful, the original document is deleted.

- The `input.pdf` file is converted to a set of image files using `pdftoppm`.

- The set of images files is OCR-ed and processed into a PDF using `tesseract`.

- When the job is deleted, all it's files are overwritten and then deleted.

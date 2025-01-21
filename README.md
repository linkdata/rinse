# rinse

Web service that converts untrusted documents to image-based PDF:s in a sandbox.

Provides both a Web UI and a Swagger REST API.

![rinse-screenshot](https://github.com/user-attachments/assets/3ff19728-beb5-4354-a2f3-7ba9fdeee424)

## Requirements

* [podman](https://podman.io/) is required.

## Running

You should start the container in [rootless](https://github.com/containers/podman/blob/main/docs/tutorials/rootless_tutorial.md) mode
with a read-only root filesystem. Inside the container we use [gVisor](https://gvisor.dev/) to further sandbox operations, and
because gVisor requires the container to be started with `--cap-add=CAP_SYS_CHROOT`, we must add that argument. If you're using SELinux,
podman will also need the `--security-opt label=type:container_engine_t` argument.

If you want the service to remember it's settings between runs, you'll need to mount a volume at `/etc/rinse` inside the container. More on this [here](#using-config-file).

`podman run --read-only --cap-drop=ALL --cap-add=CAP_SYS_CHROOT --rm -d -p 8080:8080 -v $HOME:/etc/rinse ghcr.io/linkdata/rinse`

Running it with HTTPS requires you to provide valid certificates. Rinse will look for
`fullchain.pem` and `privkey.pem` at `/etc/certs` inside the container, and if found
start in HTTPS mode.

`podman run --read-only --cap-drop=ALL --cap-add=CAP_SYS_CHROOT --rm -d -p 8443:8443 -v $HOME:/etc/rinse -v $HOME/certs:/etc/certs ghcr.io/linkdata/rinse`

### *Using config file*
The service will look for a config file named `rinse.json` in its data directory (datadir) to use when configuring its settings. The default path for datadir is `/etc/rinse` for regular users and `/var/rinse/` for root users, so the default config file path inside the container is `/etc/rinse/rinse.json`. If you wish to use a config file you need to either change the run script - either by adding another volume to mount or by create the mounted directory (`/tmp/rinse`) and placing the config within before running the script.

In this config file the following may be added:
|Settings||||
| -- | -- | -- | -- |
| *What* | *Type* | *Default* | *Adjustable*\* |
| MaxSizeMB        |int| 2048 | yes |
|	CleanupSec      |int| 86400 | yes |
|	MaxTimeSec      |int| 86400 | yes |
| TimeoutSec      |int| 600 | yes |
|	MaxConcurrent   |int| 2 | yes |
|	CleanupGotten   |bool| True | yes |
|	OAuth2          |JawsAuth.Config (nested)| - | - |
|	ProxyURL        |string| - | yes |
|	Admins          |[]string| - | yes |
|	EndpointForJWKs |string| - | - |

\* Can be changed during runtime by admins 

See [JawsAuth](https://github.com/linkdata/jawsauth) or [jawsauth/config.go](https://github.com/linkdata/jawsauth/blob/main/config.go) specifically for details on JawsAuth.Config.

## REST API

The container image will by default start `/usr/bin/rinse`, but it also provides a development version you can use by
overriding the entrypoint with `--entrypoint /usr/bin/rinse-devel`. This version contains the full Swagger UI.

## Process

First, a temporary directory is created for the job. This will be mounted in the 
gVisor container as `/var/rinse`. If we were given an URL, we download the
document and place it here.

Then, each of these stages run in their own gVisor container, which is destroyed 
as soon as the stage is complete or fails. When the job is removed, all it's files
are overwritten before they are deleted from the container filesystem.

- We extract metadata about the document using [Apache Tika](https://tika.apache.org/)
  and save it with the document file name plus `.json`.

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


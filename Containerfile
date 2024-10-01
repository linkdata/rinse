FROM alpine:latest AS rinseworker
LABEL org.opencontainers.image.source="https://github.com/linkdata/rinse"

RUN apk --no-cache -U upgrade && apk --no-cache add \
    msttcorefonts-installer \
    fontconfig \
    poppler-utils \
    openjdk8 \
    libreoffice \
    ttf-cantarell \
    ttf-dejavu \
    ttf-droid \
    ttf-font-awesome \
    ttf-freefont \
    ttf-hack \
    ttf-inconsolata \
    ttf-liberation \
    ttf-linux-libertine \
    ttf-mononoki \
    ttf-opensans \
    font-noto-cjk \
    icu-data-full \
    tesseract-ocr \
    tesseract-ocr-data-afr \
    tesseract-ocr-data-ara \
    tesseract-ocr-data-aze \
    tesseract-ocr-data-bel \
    tesseract-ocr-data-ben \
    tesseract-ocr-data-bul \
    tesseract-ocr-data-cat \
    tesseract-ocr-data-ces \
    tesseract-ocr-data-chi_sim \
    tesseract-ocr-data-chi_tra \
    tesseract-ocr-data-chr \
    tesseract-ocr-data-dan \
    tesseract-ocr-data-deu \
    tesseract-ocr-data-eng \
    tesseract-ocr-data-enm \
    tesseract-ocr-data-epo \
    tesseract-ocr-data-equ \
    tesseract-ocr-data-est \
    tesseract-ocr-data-eus \
    tesseract-ocr-data-fin \
    tesseract-ocr-data-fra \
    tesseract-ocr-data-frk \
    tesseract-ocr-data-frm \
    tesseract-ocr-data-glg \
    tesseract-ocr-data-grc \
    tesseract-ocr-data-heb \
    tesseract-ocr-data-hin \
    tesseract-ocr-data-hrv \
    tesseract-ocr-data-hun \
    tesseract-ocr-data-ind \
    tesseract-ocr-data-isl \
    tesseract-ocr-data-ita \
    tesseract-ocr-data-jpn \
    tesseract-ocr-data-kan \
    tesseract-ocr-data-kat \
    tesseract-ocr-data-khm \
    tesseract-ocr-data-kor \
    tesseract-ocr-data-lav \
    tesseract-ocr-data-lit \
    tesseract-ocr-data-mal \
    tesseract-ocr-data-mkd \
    tesseract-ocr-data-mlt \
    tesseract-ocr-data-msa \
    tesseract-ocr-data-nld \
    tesseract-ocr-data-nor \
    tesseract-ocr-data-osd \
    tesseract-ocr-data-pol \
    tesseract-ocr-data-por \
    tesseract-ocr-data-ron \
    tesseract-ocr-data-rus \
    tesseract-ocr-data-slk \
    tesseract-ocr-data-slv \
    tesseract-ocr-data-spa \
    tesseract-ocr-data-sqi \
    tesseract-ocr-data-srp \
    tesseract-ocr-data-swa \
    tesseract-ocr-data-swe \
    tesseract-ocr-data-tam \
    tesseract-ocr-data-tel \
    tesseract-ocr-data-tgl \
    tesseract-ocr-data-tha \
    tesseract-ocr-data-tur \
    tesseract-ocr-data-ukr \
    tesseract-ocr-data-vie

RUN update-ms-fonts
RUN wget -O /usr/local/bin/tika.jar https://archive.apache.org/dist/tika/2.9.2/tika-app-2.9.2.jar
    
COPY tesseract_opencl_profile_devices.dat /

RUN addgroup -g 1000 rinse && \
    adduser -u 1000 -s /bin/true -G rinse -h /var/rinse -D rinse && \
    mkdir -p /var/rinse && \
    chmod 777 /var/rinse

WORKDIR /

#############################

FROM alpine:latest AS rinse
LABEL org.opencontainers.image.source="https://github.com/linkdata/rinse"

RUN apk --no-cache -U upgrade

RUN GVISOR=https://storage.googleapis.com/gvisor/releases/release/latest/$(uname -m) && \
    wget ${GVISOR}/runsc ${GVISOR}/runsc.sha512 && \
    sha512sum -c runsc.sha512 && \
    rm -f *.sha512 && \
    chmod a+rx runsc && \
    mv runsc /usr/bin

COPY rinse /usr/bin/rinse
RUN chmod 555 /usr/bin/rinse

RUN addgroup -g 1000 rinse && \
    adduser -u 1000 -s /bin/true -G rinse -h /home/rinse -D rinse

RUN mkdir /var/run/runsc && chmod 777 /var/run/runsc
RUN mkdir /var/rinse && chmod 777 /var/rinse
RUN mkdir /opt/rinseworker && chmod 555 /opt/rinseworker
COPY --from=rinseworker / /opt/rinseworker

ENV RINSE_CERTDIR=
ENV RINSE_LISTEN=
ENV RINSE_USER=
ENTRYPOINT /usr/bin/rinse

# CGO_ENABLED=0 go build .
# podman build .
# podman run --rm -v /proc:/newproc:ro -p 8080:80 --cap-add SYS_ADMIN -it ghcr.io/linkdata/rinse
# sudo podman run --rm -u=www-data -v /proc:/newproc:ro -p 443:443 -v ~/certs:/etc/certs --env RINSE_CERTDIR=/etc/certs --cap-add SYS_ADMIN -it ghcr.io/linkdata/rinse

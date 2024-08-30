# podman build -t rinse .

FROM alpine:latest

RUN apk --no-cache -U upgrade && \
    apk --no-cache add poppler-utils tesseract-ocr

RUN addgroup rinse && \
    adduser -s /bin/true -G rinse -h /home/rinse -D rinse

USER rinse

# COPY rinse /
# ENTRYPOINT ["/rinse"]

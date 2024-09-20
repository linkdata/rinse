package rinser

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strconv"

	contentnegotiation "gitlab.com/jamietanna/content-negotiation-go"
)

// RESTGETJobsUUIDPreview godoc
//
//	@Summary		Show a job preview image
//	@Description	show job preview image by UUID
//	@Tags			jobs
//	@Accept			*/*
//	@Produce		html
//	@Produce		jpeg
//	@Param			uuid	path		string	true	"49d1e304-d2b8-46bf-b6a6-f1e9b797e1b0"
//	@Param			pages	query		int		false	"1"
//	@Param			width	query		int		false	"172"
//	@Success		200		{html}		html	""
//	@Success		200		{jpeg}		jpeg	""
//	@Success		202		{object}	Job		"Preview not yet ready."
//	@Failure		400		{object}	HTTPError
//	@Failure		404		{object}	HTTPError
//	@Failure		410		{object}	HTTPError		"Job failed."
//	@Failure		500		{object}	HTTPError
//	@Router			/jobs/{uuid}/preview [get]
func (rns *Rinse) RESTGETJobsUUIDPreview(w http.ResponseWriter, r *http.Request) {
	const iframeStart = `<!DOCTYPE html><html><body><img alt="%s" src="data:image/jpeg;base64,`
	const iframeEnd = `" width="%dpx"></body></html>`
	if job := rns.FindJob(r.PathValue("uuid")); job != nil {
		switch job.State() {
		case JobNew:
			HTTPJSON(w, http.StatusAccepted, job)
			return
		case JobFailed:
			SendHTTPError(w, http.StatusGone, job.Error)
			return
		default:
			negotiator := contentnegotiation.NewNegotiator("image/jpeg", "text/html")
			negotiated, _, err := negotiator.Negotiate(r.Header.Get("Accept"))
			if err == nil {
				numPages := 1
				imgWidth := 172

				iframe := negotiated.String() == "text/html"

				if s := r.URL.Query().Get("pages"); s != "" {
					if n, e := strconv.Atoi(s); e == nil {
						numPages = n
					}
				}

				if s := r.URL.Query().Get("width"); s != "" {
					if n, e := strconv.Atoi(s); e == nil {
						imgWidth = n
					}
				}

				var b []byte
				if b, err = job.Preview(numPages, imgWidth); err == nil {

					if b == nil {
						HTTPJSON(w, http.StatusAccepted, job)
						return
					}

					hdr := w.Header()
					if iframe {
						var buf bytes.Buffer
						if _, err = fmt.Fprintf(&buf, iframeStart, html.EscapeString(job.DocumentName())); err == nil {
							wc := base64.NewEncoder(base64.RawStdEncoding, &buf)
							if _, err = wc.Write(b); err == nil {
								if err = wc.Close(); err == nil {
									if _, err = fmt.Fprintf(&buf, iframeEnd, imgWidth); err == nil {
										hdr["Content-Length"] = []string{strconv.Itoa(buf.Len())}
										hdr["Content-Type"] = []string{"text/html"}
										if _, err = w.Write(buf.Bytes()); err == nil {
											return
										}
									}
								}
							}
						}
					} else {
						hdr["Content-Length"] = []string{strconv.Itoa(len(b))}
						hdr["Content-Type"] = []string{"image/jpeg"}
						if _, err = w.Write(b); err == nil {
							return
						}
					}
				}
				slog.Error("handleGetPreview", "job", job.Name, "err", err)
				SendHTTPError(w, http.StatusInternalServerError, err)
				return
			}
			SendHTTPError(w, http.StatusBadRequest, err)
			return
		}
	}
	SendHTTPError(w, http.StatusNotFound, nil)
}

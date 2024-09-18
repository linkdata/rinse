package rinse

import (
	"encoding/base64"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strconv"
)

func (rns *Rinse) handleGetPreview(w http.ResponseWriter, r *http.Request) {
	if job := rns.findJob(r.PathValue("uuid")); job != nil {
		switch job.State() {
		case JobNew:
			w.WriteHeader(http.StatusAccepted)
		case JobFailed:
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			numPages := 1
			imgWidth := 172
			iframe := false

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

			if r.URL.Query().Has("iframe") {
				iframe = true
			}

			b, err := job.Preview(numPages, imgWidth)
			if err == nil {

				if b == nil {
					w.WriteHeader(http.StatusAccepted)
				}
				if iframe {
					if _, err = fmt.Fprintf(w, `<!DOCTYPE html><html><body><img alt="%s" src="data:image/jpeg;base64,`,
						html.EscapeString(job.DocumentName())); err == nil {
						wc := base64.NewEncoder(base64.RawStdEncoding, w)
						if _, err = wc.Write(b); err == nil {
							if err = wc.Close(); err == nil {
								if _, err = fmt.Fprintf(w, `" width="%dpx"></body></html>`, imgWidth); err == nil {
									return
								}
							}
						}
					}
				} else {
					hdr := w.Header()
					hdr["Content-Length"] = []string{strconv.Itoa(len(b))}
					hdr["Content-Type"] = []string{"image/jpeg"}
					if _, err = w.Write(b); err == nil {
						return
					}
				}
			}
			slog.Error("handleGetPreview", "job", job.Name, "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

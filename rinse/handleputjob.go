package rinse

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
)

var ErrMissingExtension = errors.New("file name missing extension")
var ErrNotHTTPScheme = errors.New("scheme not http(s)")

func mustHaveExtension(s string) error {
	if filepath.Ext(s) == "" {
		return ErrMissingExtension
	}
	return nil
}

func mustHaveHTTPScheme(scheme string) error {
	switch scheme {
	case "http", "https":
		return nil
	}
	return ErrNotHTTPScheme
}

func (rns *Rinse) handlePutJob(w http.ResponseWriter, r *http.Request) {
	var err error
	var job *Job
	if err = mustNotBeContentEncoded(r); err == nil {
		srcFile := filepath.Base(r.URL.Query().Get(FormFileKey))
		srcLang := r.URL.Query().Get(FormLangKey)
		srcUrl := r.URL.Query().Get(FormURLKey)
		if srcUrl != "" {
			var u *url.URL
			if u, err = url.Parse(srcUrl); err == nil {
				if err = mustHaveHTTPScheme(u.Scheme); err == nil {
					job, err = NewJob(rns, u.String(), srcLang)
				}
			}
		} else if srcFile != "." {
			if err = mustHaveExtension(srcFile); err == nil {
				if job, err = rns.NewJob(srcFile, srcLang); err == nil {
					var f *os.File
					fpath := filepath.Clean(path.Join(job.Workdir, srcFile))
					if f, err = os.Create(fpath); err == nil {
						defer f.Close()
						if _, err = io.Copy(f, r.Body); err == nil {
							err = f.Sync()
						}
					}
				}
			}
		}
	}

	if job != nil {
		if err == nil {
			if err = rns.AddJob(job); err == nil {
				if _, err = fmt.Fprintf(w, "%s\n", job.UUID.String()); err == nil {
					return
				}
			}
		}
		job.Close()
	}
	if err != nil {
		slog.Error("handlePutJob", "err", err)
	}
	w.WriteHeader(http.StatusBadRequest)
}

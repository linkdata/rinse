package rinser

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
)

const FormFileKey = "file"
const FormLangKey = "lang"
const FormURLKey = "url"

var ErrContentEncoded = errors.New("Content-Encoding is set")

func (rns *Rinse) FormFileKey() string {
	return FormFileKey
}

func (rns *Rinse) FormLangKey() string {
	return FormLangKey
}

func (rns *Rinse) FormURLKey() string {
	return FormURLKey
}

func mustNotBeContentEncoded(r *http.Request) error {
	if r.Header.Get("Content-Encoding") == "" {
		return nil
	}
	return ErrContentEncoded
}

func (rns *Rinse) handlePost(interactive bool, w http.ResponseWriter, r *http.Request) {
	srcLang := r.FormValue(FormLangKey)
	srcUrl := r.FormValue(FormURLKey)
	srcFormFile, info, err := r.FormFile(FormFileKey)
	returnUrl := "/"
	if r.FormValue("testing") == "1" {
		interactive = true
		returnUrl = "/api/"
	}

	rns.mu.Lock()
	maxSizeMB := rns.maxSizeMB
	maxTimeSec := rns.maxTimeSec
	cleanupSec := rns.cleanupSec
	timeoutSec := rns.timeoutSec
	cleanupGotten := rns.cleanupGotten
	rns.mu.Unlock()

	email := rns.GetEmail(r)

	var job *Job
	if err == nil && info != nil {
		if err = mustNotBeContentEncoded(r); err == nil {
			srcName := filepath.Base(info.Filename)
			srcFile := srcFormFile.(io.ReadCloser)

			if maxUploadSize := int64(maxSizeMB) * 1024 * 1024; maxUploadSize > 0 {
				srcFile = http.MaxBytesReader(w, srcFile, maxUploadSize)
			}
			defer srcFile.Close()

			if job, err = NewJob(rns, srcName, srcLang, maxSizeMB, maxTimeSec, cleanupSec, timeoutSec, cleanupGotten, false, email); err == nil {
				dstName := filepath.Clean(path.Join(job.Datadir, srcName))
				var dstFile *os.File
				if dstFile, err = os.Create(dstName); err == nil {
					defer dstFile.Close()
					if _, err = io.Copy(dstFile, srcFile); err == nil {
						err = dstFile.Sync()
					}
				}
			}
		}
	} else if srcUrl != "" {
		var u *url.URL
		if u, err = url.Parse(srcUrl); err == nil {
			job, err = NewJob(rns, u.String(), srcLang, maxSizeMB, maxTimeSec, cleanupSec, timeoutSec, cleanupGotten, false, email)
		}
	}

	if job != nil {
		if err == nil {
			if err = rns.AddJob(job); err == nil {
				if interactive {
					w.Header().Add("Location", returnUrl)
					w.WriteHeader(http.StatusFound)
				}
				return
			}
		}
		job.Close(err)
	}

	rns.Error("handlePost", "err", err)
	if interactive {
		rns.Jaws.Handler("error.html", errorHTML{Rinse: rns, Error: err}).ServeHTTP(w, r)
		return
	}
	w.WriteHeader(http.StatusBadRequest)
}

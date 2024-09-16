package rinse

import (
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

const FormFileKey = "file"
const FormLangKey = "lang"
const FormURLKey = "url"

func (rns *Rinse) FormFileKey() string {
	return FormFileKey
}

func (rns *Rinse) FormLangKey() string {
	return FormLangKey
}

func (rns *Rinse) FormURLKey() string {
	return FormURLKey
}

func (rns *Rinse) createJob(srcName, srcLang string, srcFile io.ReadCloser) (err error) {
	var job *Job
	if job, err = NewJob(rns, srcName, srcLang); err == nil {
		dstName := path.Join(job.Workdir, srcName)
		var dstFile *os.File
		if dstFile, err = os.Create(dstName); err == nil { // #nosec G304
			defer dstFile.Close()
			if _, err = io.Copy(dstFile, srcFile); err == nil {
				if err = rns.AddJob(job); err != nil {
					rns.Jaws.Alert("danger", err.Error())
				}
				return
			}
		}
		job.Close()
	}
	return
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
	var srcName string
	var srcFile io.ReadCloser

	if err == nil && info != nil {
		if r.Header.Get("Content-Encoding") == "" {
			srcName = filepath.Base(info.Filename)
			srcFile = srcFormFile
			defer srcFile.Close()
		}
	} else if srcUrl != "" {
		var resp *http.Response
		if resp, err = http.Get(srcUrl); err == nil { // #nosec G107
			srcName = path.Base(resp.Request.URL.Path)
			srcFile = resp.Body
			if cd := resp.Header.Get("Content-Disposition"); cd != "" {
				if _, params, e := mime.ParseMediaType(cd); e == nil {
					if s, ok := params["filename"]; ok {
						srcName = s
					}
				}
			}
			if filepath.Ext(srcName) == "" {
				if ct := resp.Header.Get("Content-Type"); ct != "" {
					if mediatype, _, e := mime.ParseMediaType(ct); e == nil {
						if exts, e := mime.ExtensionsByType(mediatype); e == nil {
							srcName += exts[0]
						}
					}
				}
			}
		}
	}

	if err == nil {
		if maxUploadSize := rns.MaxUploadSize(); maxUploadSize > 0 {
			srcFile = http.MaxBytesReader(w, srcFile, maxUploadSize)
		}
		if err = rns.createJob(srcName, srcLang, srcFile); err == nil {
			if interactive {
				w.Header().Add("Location", returnUrl)
				w.WriteHeader(http.StatusFound)
			}
			return
		}
	}

	slog.Error("handlePost", "err", err)
	if interactive {
		rns.Jaws.Handler("error.html", errorHTML{Rinse: rns, Error: err}).ServeHTTP(w, r)
	}
	w.WriteHeader(http.StatusBadRequest)
}

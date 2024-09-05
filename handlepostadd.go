package rinse

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

const FormFileKey = "formFile"
const FormLangKey = "formLang"

func (rns *Rinse) FormFileKey() string {
	return FormFileKey
}

func (rns *Rinse) FormLangKey() string {
	return FormLangKey
}

func (rns *Rinse) handlePostAdd(w http.ResponseWriter, r *http.Request) {
	srcLang := r.URL.Query().Get(FormLangKey)
	srcFormFile, info, err := r.FormFile(FormFileKey)

	if err == nil {
		srcName := filepath.Base(info.Filename)
		srcFile := http.MaxBytesReader(w, srcFormFile, rns.MaxUploadSize)
		defer srcFile.Close()
		var job *Job
		if job, err = NewJob(rns, srcName, srcLang); err == nil {
			dstName := path.Join(job.Workdir, srcName)
			var dstFile *os.File
			if dstFile, err = os.Create(dstName); err == nil {
				defer dstFile.Close()
				if _, err = io.Copy(dstFile, srcFile); err == nil {
					rns.AddJob(job)
					w.Header().Add("Location", "/")
					w.WriteHeader(http.StatusFound)
					return
				}
			}
			job.Close()
		}
	}
	slog.Error("handlePostAdd", "err", err)
	w.WriteHeader(http.StatusInternalServerError)
}

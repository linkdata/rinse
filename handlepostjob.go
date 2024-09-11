package rinse

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

const FormFileKey = "file"
const FormLangKey = "lang"

func (rns *Rinse) FormFileKey() string {
	return FormFileKey
}

func (rns *Rinse) FormLangKey() string {
	return FormLangKey
}

func (rns *Rinse) handlePostJob(w http.ResponseWriter, r *http.Request) {
	srcFormFile, info, err := r.FormFile(FormFileKey)
	srcLang := r.FormValue(FormLangKey)

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
					if err = rns.AddJob(job); err != nil {
						rns.Jaws.Alert("danger", err.Error())
					}
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

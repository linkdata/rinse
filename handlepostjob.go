package rinse

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

func (rns *Rinse) handlePostJob(w http.ResponseWriter, r *http.Request) {
	srcFormFile, info, err := r.FormFile(FormFileKey)
	srcLang := r.FormValue(FormLangKey)
	status := http.StatusBadRequest

	if err == nil {
		srcName := filepath.Base(info.Filename)
		if filepath.Ext(srcName) != "" {
			srcFile := http.MaxBytesReader(w, srcFormFile, rns.MaxUploadSize())
			defer srcFile.Close()
			var job *Job
			if job, err = NewJob(rns, srcName, srcLang); err == nil {
				status = http.StatusInternalServerError
				dstName := path.Join(job.Workdir, srcName)
				var dstFile *os.File
				if dstFile, err = os.Create(dstName); err == nil {
					defer dstFile.Close()
					if _, err = io.Copy(dstFile, srcFile); err == nil {
						if err = dstFile.Sync(); err == nil {
							if err = rns.AddJob(job); err == nil {
								if _, err = fmt.Fprintf(w, "%s\n", job.UUID.String()); err == nil {
									return
								}
							}
						}
					}
				}
				job.Close()
			}
		}
	}
	if err != nil {
		slog.Error("handlePostJob", "err", err)
	}
	w.WriteHeader(status)
}

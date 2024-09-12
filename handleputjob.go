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

func (rns *Rinse) handlePutJob(w http.ResponseWriter, r *http.Request) {
	status := http.StatusBadRequest
	if r.Header.Get("Content-Encoding") == "" {
		if name := filepath.Base(r.PathValue("file")); name != "" {
			if ext := filepath.Ext(name); ext != "" {
				status = http.StatusInternalServerError
				job, err := rns.NewJob(name, r.URL.Query().Get("lang"))
				if err == nil {
					var f *os.File
					if f, err = os.Create(path.Join(job.Workdir, name)); err == nil {
						defer f.Close()
						if _, err = io.Copy(f, r.Body); err == nil {
							if err = f.Sync(); err == nil {
								if err = rns.AddJob(job); err == nil {
									fmt.Fprintf(w, "%s\n", job.UUID.String())
									return
								}
							}
						}
					}
					job.Close()
				}
				slog.Error("handlePutJob", "err", err)
			}
		}
	}
	w.WriteHeader(status)
}

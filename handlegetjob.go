package rinse

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/google/uuid"
)

func (rns *Rinse) findJob(u uuid.UUID) *Job {
	rns.mu.Lock()
	defer rns.mu.Unlock()
	for _, job := range rns.jobs {
		if job.UUID == u {
			return job
		}
	}
	return nil
}

func (rns *Rinse) handleGetJob(w http.ResponseWriter, r *http.Request) {
	if s := r.PathValue("uuid"); s != "" {
		u, err := uuid.Parse(s)
		if err == nil {
			if job := rns.findJob(u); job != nil {
				switch job.State() {
				case JobFailed:
					w.WriteHeader(http.StatusNoContent)
					return
				case JobFinished:
					var fi os.FileInfo
					if fi, err = os.Stat(job.ResultPath()); err == nil {
						hdr := w.Header()
						hdr["Content-Length"] = []string{strconv.FormatInt(fi.Size(), 10)}
						hdr["Content-Type"] = []string{"application/pdf"}
						hdr["Content-Disposition"] = []string{fmt.Sprintf(`attachment; filename="%s"`, job.ResultName)}
						w.WriteHeader(http.StatusOK)
						var f *os.File
						if f, err = os.Open(job.ResultPath()); err == nil {
							defer f.Close()
							if _, err = io.Copy(w, f); err == nil {
								return
							}
						}
					}
				default:
					w.WriteHeader(http.StatusAccepted)
					return
				}
			} else {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			slog.Error("handleGetJob", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusBadRequest)
}

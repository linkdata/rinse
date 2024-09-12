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

func (rns *Rinse) findJobUuid(u uuid.UUID) *Job {
	rns.mu.Lock()
	defer rns.mu.Unlock()
	for _, job := range rns.jobs {
		if job.UUID == u {
			return job
		}
	}
	return nil
}

func (rns *Rinse) findJob(s string) *Job {
	if s != "" {
		if u, err := uuid.Parse(s); err == nil {
			return rns.findJobUuid(u)
		}
	}
	return nil
}

func (rns *Rinse) handleGetJob(w http.ResponseWriter, r *http.Request) {
	if job := rns.findJob(r.PathValue("uuid")); job != nil {
		switch job.State() {
		case JobFailed:
			w.WriteHeader(http.StatusNoContent)
			return
		case JobFinished:
			fi, err := os.Stat(job.ResultPath())
			if err == nil {
				hdr := w.Header()
				hdr["Content-Length"] = []string{strconv.FormatInt(fi.Size(), 10)}
				hdr["Content-Type"] = []string{"application/pdf"}
				hdr["Content-Disposition"] = []string{fmt.Sprintf(`attachment; filename="%s"`, job.ResultName)}
				var f *os.File
				if f, err = os.Open(job.ResultPath()); err == nil {
					defer f.Close()
					if _, err = io.Copy(w, f); err == nil {
						return
					}
				}
			}
			slog.Error("handleGetJob", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusAccepted)
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

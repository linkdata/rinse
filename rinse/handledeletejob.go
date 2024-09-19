package rinse

import (
	"net/http"
)

func (rns *Rinse) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	if job := rns.findJob(r.PathValue("uuid")); job != nil {
		rns.RemoveJob(job)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

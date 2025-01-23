package rinser

import "net/http"

// RESTDELETEJobsUUID godoc
//
//	@Summary		Delete a job
//	@Description	Delete by job UUID
//	@Tags			jobs
//	@Accept			*/*
//	@Produce		json
//	@Param			uuid			path		string	true	"49d1e304-d2b8-46bf-b6a6-f1e9b797e1b0"
//	@Param			Authorization	header		string	false	"JWT token"
//	@Success		200				{object}	Job
//	@Failure		404				{object}	HTTPError
//	@Router			/jobs/{uuid} [delete]
func (rns *Rinse) RESTDELETEJobsUUID(hw http.ResponseWriter, hr *http.Request) {
	if job := rns.FindJob(hr.PathValue("uuid")); job != nil {
		rns.RemoveJob(job)
		HTTPJSON(hw, http.StatusOK, job)
	} else {
		SendHTTPError(hw, http.StatusNotFound, nil)
	}
}

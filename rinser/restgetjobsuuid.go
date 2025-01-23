package rinser

import "net/http"

// RESTGETJobsUUID godoc
//
//	@Summary		Get job metadata.
//	@Description	Get job metadata by UUID.
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Param			uuid			path		string	true	"49d1e304-d2b8-46bf-b6a6-f1e9b797e1b0"
//	@Param			Authorization	header		string	false	"JWT token"
//	@Success		200				{object}	Job
//	@Failure		404				{object}	HTTPError
//	@Router			/jobs/{uuid} [get]
func (rns *Rinse) RESTGETJobsUUID(hw http.ResponseWriter, hr *http.Request) {
	if job := rns.FindJob(hr.PathValue("uuid")); job != nil {
		HTTPJSON(hw, http.StatusOK, job)
	} else {
		SendHTTPError(hw, http.StatusNotFound, nil)
	}
}

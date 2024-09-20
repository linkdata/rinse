package rinser

import "net/http"

// RESTGETJobs godoc
//
//	@Summary		List jobs
//	@Description	Get a list of all jobs.
//	@Tags			jobs
//	@Accept			*/*
//	@Produce		json
//	@Success		200	{array}	Job
//	@Router			/jobs [get]
func (rns *Rinse) RESTGETJobs(hw http.ResponseWriter, hr *http.Request) {
	HTTPJSON(hw, http.StatusOK, rns.JobList())
}

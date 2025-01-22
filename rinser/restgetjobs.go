package rinser

import "net/http"

// RESTGETJobs godoc
//
//	@Summary		List jobs
//	@Description	Get a list of all jobs.
//	@Tags			jobs
//	@Accept			*/*
//	@Produce		json
//	@Param			Authorization	header	string	false	"JWT token"
//	@Success		200				{array}	Job
//	@Router			/jobs [get]
func (rns *Rinse) RESTGETJobs(hw http.ResponseWriter, hr *http.Request) {
	list := rns.JobList(rns.GetEmail(hr))
	if list == nil {
		list = []*Job{}
	}
	HTTPJSON(hw, http.StatusOK, list)
}

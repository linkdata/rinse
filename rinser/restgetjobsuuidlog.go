package rinser

import (
	"io"
	"net/http"
	"os"
	"strconv"
)

// RESTGETJobsUUIDLog godoc
//
//	@Summary		Get the jobs log.
//	@Description	Get the jobs log.
//	@Tags			jobs
//	@Accept			*/*
//	@Produce		text/plain
//	@Param			uuid	path		string	true	"49d1e304-d2b8-46bf-b6a6-f1e9b797e1b0"
//	@Success		200		{file}		file	""
//	@Success		202		{object}	Job		"Log not yet ready."
//	@Failure		404		{object}	HTTPError
//	@Failure		410		{object}	HTTPError "Job failed."
//	@Failure		500		{object}	HTTPError
//	@Router			/jobs/{uuid}/log [get]
func (rns *Rinse) RESTGETJobsUUIDLog(hw http.ResponseWriter, hr *http.Request) {
	if job := rns.FindJob(hr.PathValue("uuid")); job != nil {
		if job.State() == JobFailed {
			SendHTTPError(hw, http.StatusGone, job.Error)
			return
		}
		if job.HasLog() {
			fi, err := os.Stat(job.LogPath())
			if err == nil {
				hdr := hw.Header()
				hdr["Content-Length"] = []string{strconv.FormatInt(fi.Size(), 10)}
				hdr["Content-Type"] = []string{"text/plain; charset=utf-8"}
				var f *os.File
				if f, err = os.Open(job.LogPath()); err == nil /* #nosec G304 */ {
					defer f.Close()
					if _, err = io.Copy(hw, f); err == nil {
						return
					}
				}
			}
			rns.Error("RESTGETJobsUUIDLog", "job", job.Name, "err", err)
			SendHTTPError(hw, http.StatusInternalServerError, err)
		} else {
			HTTPJSON(hw, http.StatusAccepted, job)
		}
	} else {
		SendHTTPError(hw, http.StatusNotFound, nil)
	}
}

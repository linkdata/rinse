package rinser

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
)

// RESTGETJobsUUIDRinsed godoc
//
//	@Summary		Get the jobs rinsed document.
//	@Description	Get the jobs rinsed document.
//	@Tags			jobs
//	@Accept			*/*
//	@Produce		application/pdf
//	@Produce		json
//	@Param			uuid	path		string	true	"49d1e304-d2b8-46bf-b6a6-f1e9b797e1b0"
//	@Success		200		{file}		file	""
//	@Success		202		{object}	Job		"Rinsed version not yet ready."
//	@Failure		404		{object}	HTTPError
//	@Failure		410		{object}	HTTPError "Job failed."
//	@Failure		500		{object}	HTTPError
//	@Router			/jobs/{uuid}/rinsed [get]
func (rns *Rinse) RESTGETJobsUUIDRinsed(hw http.ResponseWriter, hr *http.Request) {
	if job := rns.FindJob(hr.PathValue("uuid")); job != nil {
		switch job.State() {
		case JobFailed:
			SendHTTPError(hw, http.StatusGone, job.Error)
			return
		case JobFinished:
			fi, err := os.Stat(job.ResultPath())
			if err == nil {
				hdr := hw.Header()
				hdr["Content-Length"] = []string{strconv.FormatInt(fi.Size(), 10)}
				hdr["Content-Type"] = []string{"application/pdf"}
				hdr["Content-Disposition"] = []string{fmt.Sprintf(`attachment; filename="%s"`, job.ResultName())}
				var f *os.File
				if f, err = os.Open(job.ResultPath()); err == nil {
					defer f.Close()
					if _, err = io.Copy(hw, f); err == nil {
						return
					}
				}
			}
			slog.Error("RESTGETJobsUUIDRinsed", "job", job.Name, "err", err)
			SendHTTPError(hw, http.StatusInternalServerError, err)
		default:
			HTTPJSON(hw, http.StatusAccepted, job)
		}
	} else {
		SendHTTPError(hw, http.StatusNotFound, nil)
	}
}

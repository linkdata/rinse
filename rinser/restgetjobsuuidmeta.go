package rinser

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strconv"
)

// RESTGETJobsUUIDMeta godoc
//
//	@Summary		Get the jobs document metadata.
//	@Description	Get the jobs document metadata.
//	@Tags			jobs
//	@Accept			*/*
//	@Produce		json
//	@Param			uuid	path		string	true	"49d1e304-d2b8-46bf-b6a6-f1e9b797e1b0"
//	@Success		200		{file}		file	""
//	@Success		202		{object}	Job		"Metadata not yet ready."
//	@Failure		404		{object}	HTTPError
//	@Failure		410		{object}	HTTPError "Job failed."
//	@Failure		500		{object}	HTTPError
//	@Router			/jobs/{uuid}/meta [get]
func (rns *Rinse) RESTGETJobsUUIDMeta(hw http.ResponseWriter, hr *http.Request) {
	if job := rns.FindJob(hr.PathValue("uuid")); job != nil {
		if job.State() == JobFailed {
			SendHTTPError(hw, http.StatusGone, job.Error)
			return
		}
		if job.State() > JobExtractMeta {
			metapath := path.Join(job.Datadir, job.docName+".json")
			fi, err := os.Stat(metapath)
			if err == nil {
				hdr := hw.Header()
				hdr["Content-Length"] = []string{strconv.FormatInt(fi.Size(), 10)}
				hdr["Content-Type"] = []string{"application/json"}
				var f *os.File
				if f, err = os.Open(metapath); err == nil /* #nosec G304 */ {
					defer f.Close()
					if _, err = io.Copy(hw, f); err == nil {
						return
					}
				}
			}
			slog.Error("RESTGETJobsUUIDMeta", "err", err)
			SendHTTPError(hw, http.StatusInternalServerError, err)
		} else {
			HTTPJSON(hw, http.StatusAccepted, job)
		}
	} else {
		SendHTTPError(hw, http.StatusNotFound, nil)
	}
}

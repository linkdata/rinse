package rinser

import (
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

// RESTPOSTJobs godoc
//
//	@Summary		Add a job
//	@Description	Add job with either a file using multipart/form-data or a URL using json.
//	@Tags			jobs
//	@Accept			json
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			addjoburl	body		AddJobURL	false	"Add job by URL"
//	@Param			file		formData	file		false	"this is a test file"
//	@Param			lang		query		string		false	"eng"
//	@Success		200			{object}	Job
//	@Failure		400			{object}	HTTPError
//	@Failure		404			{object}	HTTPError
//	@Failure		415			{object}	HTTPError
//	@Failure		500			{object}	HTTPError
//	@Router			/jobs [post]
func (rns *Rinse) RESTPOSTJobs(hw http.ResponseWriter, hr *http.Request) {
	ct, _, err := mime.ParseMediaType(hr.Header.Get("Content-Type"))
	if err == nil {
		switch ct {
		case "multipart/form-data":
			srcFormFile, info, err := hr.FormFile("file")
			if err == nil {
				srcName := filepath.Base(info.Filename)
				srcFile := srcFormFile.(io.ReadCloser)
				if maxUploadSize := rns.MaxUploadSize(); maxUploadSize > 0 {
					srcFile = http.MaxBytesReader(hw, srcFile, maxUploadSize)
				}
				defer srcFile.Close()
				srcLang := hr.URL.Query().Get("lang")
				var job *Job
				if job, err = NewJob(rns, srcName, srcLang); err == nil {
					dstName := filepath.Clean(path.Join(job.Datadir, srcName))
					var dstFile *os.File
					if dstFile, err = os.Create(dstName); err == nil {
						defer dstFile.Close()
						if _, err = io.Copy(dstFile, srcFile); err == nil {
							if err = dstFile.Sync(); err == nil {
								if err = rns.AddJob(job); err == nil {
									HTTPJSON(hw, http.StatusOK, job)
									return
								}
							}
						}
					}
				}
				slog.Error("RESTPOSTJobs", "job", job.Name, "err", err)
				SendHTTPError(hw, http.StatusInternalServerError, err)
				return
			}
		case "application/json":
			if err = mustNotBeContentEncoded(hr); err == nil {
				var addJobUrl AddJobURL
				if err = ctxShouldBindJSON(hr, &addJobUrl); err == nil {
					var job *Job
					if job, err = NewJob(rns, addJobUrl.URL, addJobUrl.Lang); err == nil {
						if err = rns.AddJob(job); err == nil {
							HTTPJSON(hw, http.StatusOK, job)
							return
						}
					}
					slog.Error("RESTPOSTJobs", "job", job.Name, "err", err)
					SendHTTPError(hw, http.StatusInternalServerError, err)
					return
				}
			}
		default:
			SendHTTPError(hw, http.StatusUnsupportedMediaType, errors.New(ct))
			return
		}
	}
	SendHTTPError(hw, http.StatusBadRequest, err)
}

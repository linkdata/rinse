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
	"strconv"
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
//	@Param			maxsizemb	query		int			false	"2048"
//	@Param			maxtimesec	query		int			false	"600"
//	@Param			cleanupsec	query		int			false	"600"
//	@Success		200			{object}	Job
//	@Failure		400			{object}	HTTPError
//	@Failure		404			{object}	HTTPError
//	@Failure		415			{object}	HTTPError
//	@Failure		500			{object}	HTTPError
//	@Router			/jobs [post]
func (rns *Rinse) RESTPOSTJobs(hw http.ResponseWriter, hr *http.Request) {
	rns.mu.Lock()
	maxSizeMB := rns.maxSizeMB
	maxTimeSec := rns.maxTimeSec
	cleanupSec := rns.cleanupSec
	rns.mu.Unlock()

	if s := hr.URL.Query().Get("maxsizemb"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			maxSizeMB = v
		}
	}

	if s := hr.URL.Query().Get("maxtimesec"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			maxTimeSec = v
		}
	}

	if s := hr.URL.Query().Get("cleanupsec"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			cleanupSec = v
		}
	}

	ct, _, err := mime.ParseMediaType(hr.Header.Get("Content-Type"))
	if err == nil {
		switch ct {
		case "multipart/form-data":
			srcFormFile, info, err := hr.FormFile("file")
			if err == nil {
				srcName := filepath.Base(info.Filename)
				srcFile := srcFormFile.(io.ReadCloser)
				if maxUploadSize := int64(maxSizeMB) * 1024 * 1024; maxUploadSize > 0 {
					srcFile = http.MaxBytesReader(hw, srcFile, maxUploadSize)
				}
				defer srcFile.Close()
				srcLang := hr.URL.Query().Get("lang")
				var job *Job
				if job, err = NewJob(rns, srcName, srcLang, maxSizeMB, maxTimeSec, cleanupSec); err == nil {
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
				addJobUrl := AddJobURL{
					MaxSizeMB:  maxSizeMB,
					MaxTimeSec: maxTimeSec,
					CleanupSec: cleanupSec,
				}
				if err = ctxShouldBindJSON(hr, &addJobUrl); err == nil {
					var job *Job
					if job, err = NewJob(rns, addJobUrl.URL, addJobUrl.Lang, addJobUrl.MaxSizeMB, addJobUrl.MaxTimeSec, addJobUrl.CleanupSec); err == nil {
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

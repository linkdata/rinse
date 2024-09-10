package rinse

import (
	"bytes"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

func (job *Job) process() {
	job.mu.Lock()
	job.started = time.Now()
	job.mu.Unlock()
	defer job.processDone()
	fn, err := job.renameInput()
	if err == nil {
		if err = job.runDocToPdf(fn); err == nil {
			if err = job.runPdfToPpm(); err == nil {
				if err = job.runTesseract(); err == nil {
					if err = job.finished(); err == nil {
						return
					}
				}
			}
		}
	}
	slog.Error("job failed", "job", job.Name, "state", job.State(), "err", err)
	job.mu.Lock()
	job.state = JobFailed
	job.mu.Unlock()
}

func (job *Job) processDone() {
	job.mu.Lock()
	job.stopped = time.Now()
	job.mu.Unlock()
}

func (job *Job) renameInput() (fn string, err error) {
	fn = "input" + strings.ToLower(filepath.Ext(job.Name))
	src := path.Join(job.Workdir, job.Name)
	dst := path.Join(job.Workdir, fn)
	if err = os.Rename(src, dst); err == nil {
		err = os.Chmod(dst, 0444)
	}
	return
}

func (job *Job) waitForDocToPdf(fn string) (err error) {
	if !strings.HasSuffix(fn, ".pdf") {
		var done int32
		defer atomic.StoreInt32(&done, 1)
		go func() {
			for atomic.LoadInt32(&done) == 0 {
				time.Sleep(time.Millisecond * 500)
				job.Jaws.Dirty(uiJobStatus{job})
			}
		}()
		if err = job.podrun(nil, "libreoffice", "--headless", "--safe-mode", "--convert-to", "pdf", "--outdir", "/var/rinse", "/var/rinse/"+fn); err == nil {
			err = os.Remove(path.Join(job.Workdir, fn))
		}
	}
	return
}

func (job *Job) runDocToPdf(fn string) (err error) {
	if err = job.transition(JobStarting, JobDocToPdf); err == nil {
		err = job.waitForDocToPdf(fn)
	}
	return
}

func (job *Job) waitForPdfToPpm() (err error) {
	var done int32
	defer atomic.StoreInt32(&done, 1)
	go func() {
		for atomic.LoadInt32(&done) == 0 {
			time.Sleep(time.Millisecond * 500)
			job.refreshDiskuse()
			job.Jaws.Dirty(uiJobStatus{job})
		}
	}()
	return job.podrun(nil, "pdftoppm", "-cropbox", "/var/rinse/input.pdf", "/var/rinse/output")
}

func (job *Job) runPdfToPpm() (err error) {
	if err = job.transition(JobDocToPdf, JobPdfToPPm); err == nil {
		if err = job.waitForPdfToPpm(); err == nil {
			if err = os.Remove(path.Join(job.Workdir, "input.pdf")); err == nil {
				var outputFiles []string
				filepath.WalkDir(job.Workdir, func(fpath string, d fs.DirEntry, err error) error {
					if err == nil {
						if d.Type().IsRegular() && filepath.Ext(fpath) == ".ppm" {
							outputFiles = append(outputFiles, d.Name())
						}
					}
					return nil
				})
				if len(outputFiles) > 0 {
					sort.Strings(outputFiles)
					var outputTxt bytes.Buffer
					for _, fn := range outputFiles {
						outputTxt.WriteString("/var/rinse/")
						outputTxt.WriteString(fn)
						outputTxt.WriteByte('\n')
					}
					if err = os.WriteFile(path.Join(job.Workdir, "output.txt"), outputTxt.Bytes(), 0666); err == nil {
						job.mu.Lock()
						job.ppmtodo = outputFiles
						job.mu.Unlock()
						job.Jaws.Dirty(uiJobStatus{job})
					}
				} else {
					err = fmt.Errorf("pdftoppm created no .ppm files")
				}
			}
		}
	}
	return
}

func (job *Job) runTesseract() (err error) {
	if err = job.transition(JobPdfToPPm, JobTesseract); err == nil {
		var args []string
		args = append(args, "tesseract")
		if job.Lang != "" {
			args = append(args, "-l", job.Lang)
		}
		args = append(args, "/var/rinse/output.txt", "/var/rinse/output", "pdf")

		stdouthandler := func(s string) error {
			job.mu.Lock()
			job.ppmtodo = slices.DeleteFunc(job.ppmtodo, func(fn string) bool {
				if strings.Contains(s, fn) {
					job.ppmdone = append(job.ppmdone, fn)
					return true
				}
				return false
			})
			job.mu.Unlock()
			job.Jaws.Dirty(uiJobStatus{job})
			return nil
		}
		err = job.podrun(stdouthandler, args...)
	}
	return
}

func (job *Job) finished() (err error) {
	if err = job.transition(JobTesseract, JobFinished); err == nil {
		_ = filepath.WalkDir(job.Workdir, func(fpath string, d fs.DirEntry, err error) error {
			if err == nil {
				if d.Type().IsRegular() && d.Name() != "output.pdf" {
					_ = os.Remove(fpath)
				}
			}
			return nil
		})
		err = os.Rename(path.Join(job.Workdir, "output.pdf"), path.Join(job.Workdir, job.ResultName))
	}
	job.refreshDiskuse()
	job.Jaws.Dirty(job, uiJobStatus{job})
	return
}

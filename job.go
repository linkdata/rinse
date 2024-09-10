package rinse

import (
	"bytes"
	"context"
	"errors"
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

	"github.com/google/uuid"
	"github.com/linkdata/deadlock"
)

type JobState int

const (
	JobNew JobState = iota
	JobStarting
	JobDocToPdf
	JobPdfToPPm
	JobTesseract
	JobFinished
	JobFailed
)

type Job struct {
	*Rinse
	Name       string
	ResultName string
	Lang       string
	Workdir    string
	Created    time.Time
	UUID       uuid.UUID
	mu         deadlock.Mutex
	state      JobState
	resultCh   chan error
	started    time.Time
	stopped    time.Time
	closed     bool
	ppmtodo    []string
	ppmdone    []string
	diskuse    int64
	nfiles     int
}

var ErrIllegalLanguage = errors.New("illegal language string")

func checkLangString(lang string) error {
	for _, ch := range lang {
		if !(ch == '+' || (ch >= 'a' && ch <= 'z')) {
			return ErrIllegalLanguage
		}
	}
	return nil
}

func NewJob(rns *Rinse, name, lang string) (job *Job, err error) {
	if err = checkLangString(lang); err == nil {
		var workdir string
		if workdir, err = os.MkdirTemp("", "rinse-"); err == nil {
			if err = os.Chmod(workdir, 0777); err == nil {
				name = filepath.Base(name)
				ext := filepath.Ext(name)
				job = &Job{
					Rinse:      rns,
					Name:       name,
					ResultName: strings.TrimSuffix(name, ext) + "-rinsed.pdf",
					Lang:       lang,
					Workdir:    workdir,
					Created:    time.Now(),
					UUID:       uuid.New(),
					state:      JobNew,
					resultCh:   make(chan error, 1),
				}
			}
		}
	}
	return
}

func (job *Job) renameInput() (fn string, err error) {
	fn = "input" + strings.ToLower(filepath.Ext(job.Name))
	dst := path.Join(job.Workdir, fn)
	if err = os.Rename(path.Join(job.Workdir, job.Name), dst); err == nil {
		err = os.Chmod(dst, 0444)
	}
	return
}

func (job *Job) Start() (err error) {
	if err = job.transition(JobNew, JobStarting); err == nil {
		var fn string
		if fn, err = job.renameInput(); err == nil {
			job.mu.Lock()
			job.started = time.Now()
			job.mu.Unlock()
			if strings.HasSuffix(fn, ".pdf") {
				go job.runPdfToPpm(JobStarting)
			} else {
				go job.runDocToPdf(fn)
			}
		}
	}
	job.checkErr(err)
	return
}

func (job *Job) State() (state JobState) {
	job.mu.Lock()
	state = job.state
	job.mu.Unlock()
	return
}

func (job *Job) ResultCh() (ch <-chan error) {
	job.mu.Lock()
	ch = job.resultCh
	job.mu.Unlock()
	return
}

func (job *Job) ResultPath() string {
	return path.Join(job.Workdir, job.ResultName)
}

func (job *Job) checkErrLocked(err error) {
	if err != nil && job.state != JobFailed {
		job.state = JobFailed
		slog.Info("job failed", "job", job.Name, "err", err)
		select {
		case job.resultCh <- err:
		default:
			slog.Error("failed to signal job failed", "name", job.Name, "err", err)
		}

	}
}

func (job *Job) checkErr(err error) {
	if err != nil {
		job.mu.Lock()
		defer job.mu.Unlock()
		job.checkErrLocked(err)
	}
}

func (job *Job) transition(fromState, toState JobState) (err error) {
	job.mu.Lock()
	if job.state == fromState {
		job.state = toState
	} else {
		err = fmt.Errorf("expected job state %d, have %d", fromState, job.state)
	}
	job.mu.Unlock()
	job.refreshDiskuse()
	job.Jaws.Dirty(job, uiJobStatus{job})
	return
}

func (job *Job) podrun(stdouthandler func(string) error, cmds ...string) (err error) {
	return podrun(context.Background(), job.PodmanBin, job.RunscBin, job.Workdir, stdouthandler, cmds...)
}

/*
	libreoffice --headless --safe-mode --convert-to pdf --outdir /var/rinse /var/rinse/input.xxx

 	.docx, .doc, .docm, .xlsx, .xls, .pptx, .ppt, .odt, .odg, .odp, .ods, .ots, .ott
*/

func (job *Job) waitForDocToPdf(fn string) (err error) {
	var done int32
	defer atomic.StoreInt32(&done, 1)
	go func() {
		for atomic.LoadInt32(&done) == 0 {
			time.Sleep(time.Millisecond * 500)
			job.Jaws.Dirty(uiJobStatus{job})
		}
	}()
	return job.podrun(nil, "libreoffice", "--headless", "--safe-mode", "--convert-to", "pdf", "--outdir", "/var/rinse", "/var/rinse/"+fn)
}

func (job *Job) runDocToPdf(fn string) {
	err := job.transition(JobStarting, JobDocToPdf)
	if err == nil {
		if err = job.waitForDocToPdf(fn); err == nil {
			if err = os.Remove(path.Join(job.Workdir, fn)); err == nil {
				job.runPdfToPpm(JobDocToPdf)
				return
			}
		}
	}
	job.checkErr(err)
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

func (job *Job) runPdfToPpm(fromState JobState) {
	err := job.transition(fromState, JobPdfToPPm)

	if err == nil {
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
						job.Jaws.Dirty(uiJobStatus{job})
						job.mu.Unlock()
						if err = job.runTesseract(); err == nil {
							if err = job.transition(JobTesseract, JobFinished); err == nil {
								job.mu.Lock()
								job.stopped = time.Now()
								ch := job.resultCh
								job.mu.Unlock()
								job.Jaws.Dirty(job, uiJobStatus{job})
								select {
								case ch <- nil:
								default:
								}
							}
						}
					}
				} else {
					err = fmt.Errorf("pdftoppm created no .ppm files")
				}
			}
		}
		job.mu.Lock()
		job.stopped = time.Now()
		job.mu.Unlock()
	}

	job.checkErr(err)
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
		if err = job.podrun(stdouthandler, args...); err == nil {
			var toremove []string
			job.mu.Lock()
			for _, fn := range job.ppmdone {
				toremove = append(toremove, path.Join(job.Workdir, fn))
			}
			job.mu.Unlock()
			for _, fn := range toremove {
				_ = os.Remove(fn)
			}
			_ = os.Remove(path.Join(job.Workdir, "output.txt"))
			err = os.Rename(path.Join(job.Workdir, "output.pdf"), path.Join(job.Workdir, job.ResultName))
			job.refreshDiskuse()
			job.Jaws.Dirty(job, uiJobStatus{job})
		}
	}
	return
}

func (job *Job) Result() (err error) {
	if err = os.Rename(path.Join(job.Workdir, "output.pdf"), path.Join(job.Workdir, job.Name)); err == nil {
	}
	return
}

func (job *Job) Close() (err error) {
	defer job.RemoveJob(job)
	job.mu.Lock()
	defer job.mu.Unlock()
	if !job.closed {
		job.closed = true
		if job.state != JobFinished {
			job.state = JobFailed
		}
		close(job.resultCh)
		err = os.RemoveAll(job.Workdir)
		job.diskuse, job.nfiles = job.getDiskuse()
	}
	return
}

func (job *Job) refreshDiskuse() {
	diskuse, nfiles := job.getDiskuse()
	job.mu.Lock()
	job.diskuse = diskuse
	job.nfiles = nfiles
	job.mu.Unlock()
}

func (job *Job) getDiskuse() (diskuse int64, nfiles int) {
	filepath.WalkDir(job.Workdir, func(path string, d fs.DirEntry, err error) error {
		if err == nil {
			if fi, err := d.Info(); err == nil {
				diskuse += fi.Size()
			}
			if filepath.Ext(d.Name()) == ".ppm" {
				nfiles++
			}
		}
		return nil
	})
	return
}

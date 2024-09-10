package rinse

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
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
				}
			}
		}
	}
	return
}

func (job *Job) Start() (err error) {
	if err = job.transition(JobNew, JobStarting); err == nil {
		go job.process()
	}
	return
}

func (job *Job) State() (state JobState) {
	job.mu.Lock()
	state = job.state
	job.mu.Unlock()
	return
}

func (job *Job) ResultPath() string {
	return path.Join(job.Workdir, job.ResultName)
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

func (job *Job) Close() (err error) {
	defer job.RemoveJob(job)
	job.mu.Lock()
	defer job.mu.Unlock()
	if !job.closed {
		job.closed = true
		if job.state != JobFinished {
			job.state = JobFailed
		}
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

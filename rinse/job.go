package rinse

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
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
	JobDownload
	JobDetectLanguage
	JobDocToPdf
	JobPdfToImages
	JobTesseract
	JobEnding
	JobFinished
	JobFailed
)

type AddJobURL struct {
	URL  string `json:"url" example:"https://getsamplefiles.com/download/pdf/sample-1.pdf"`
	Lang string `json:"lang" example:"auto"`
}

type Job struct {
	*Rinse   `json:"-"`
	Workdir  string         `json:"workdir" example:"/tmp/rinse-12345678"`
	Name     string         `json:"name" example:"example.docx"`
	Created  time.Time      `json:"created" example:"2024-01-01T12:00:00+00:00" format:"dateTime"`
	UUID     uuid.UUID      `json:"uuid" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	mu       deadlock.Mutex // protects following
	Error    error          `json:"error,omitempty"`
	PdfName  string         `json:"pdfname,omitempty" example:"example-rinsed.pdf"` // rinsed PDF file name
	Language string         `json:"lang,omitempty" example:"auto"`
	started  time.Time
	stopped  time.Time
	docName  string // document file name, once known
	state    JobState
	imgfiles map[string]bool
	diskuse  int64
	cancelFn context.CancelFunc
	closed   bool
	errstate JobState
	previews map[uint64][]byte
}

var ErrIllegalLanguage = errors.New("illegal language string")

func checkLangString(lang string) error {
	for _, ch := range lang {
		if !(ch == '+' || ch == '_' || (ch >= 'a' && ch <= 'z')) {
			return ErrIllegalLanguage
		}
	}
	return nil
}

func NewJob(rns *Rinse, name, lang string) (job *Job, err error) {
	if err = checkLangString(lang); err == nil {
		if lang == "auto" {
			lang = ""
		}
		var workdir string
		if workdir, err = os.MkdirTemp("", "rinse-"); err == nil {
			if err = os.Chmod(workdir, 0777); err == nil { // #nosec G302
				job = &Job{
					Rinse:    rns,
					Name:     name,
					Language: lang,
					Workdir:  workdir,
					Created:  time.Now(),
					UUID:     uuid.New(),
					state:    JobNew,
					imgfiles: make(map[string]bool),
					previews: make(map[uint64][]byte),
				}
			}
		}
	}
	return
}

func (job *Job) Previewable() (yes bool) {
	job.mu.Lock()
	yes = len(job.imgfiles) > 0
	job.mu.Unlock()
	return
}

func (job *Job) Start(maxTime time.Duration) (err error) {
	if err = job.transition(JobNew, JobStarting); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), maxTime)
		job.mu.Lock()
		job.cancelFn = cancel
		job.mu.Unlock()
		go job.process(ctx)
	}
	return
}

func (job *Job) State() (state JobState) {
	job.mu.Lock()
	state = job.state
	job.mu.Unlock()
	return
}

func (job *Job) Lang() (s string) {
	job.mu.Lock()
	s = job.Language
	job.mu.Unlock()
	return
}

func (job *Job) DocumentName() (s string) {
	job.mu.Lock()
	s = job.docName
	job.mu.Unlock()
	return
}

func (job *Job) ResultName() (s string) {
	job.mu.Lock()
	s = job.PdfName
	job.mu.Unlock()
	return
}

func (job *Job) ResultPath() string {
	return path.Join(job.Workdir, job.ResultName())
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
	return
}

func (job *Job) podrun(ctx context.Context, stdouthandler func(string) error, cmds ...string) (err error) {
	return podrun(ctx, job.PodmanBin, job.RunscBin, job.Workdir, stdouthandler, cmds...)
}

func (job *Job) Stop() {
	job.mu.Lock()
	cancel := job.cancelFn
	if job.state != JobFinished {
		job.state = JobFailed
	}
	job.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (job *Job) removeAll() {
	if err := scrub(job.Workdir); err != nil {
		slog.Error("job.removeAll", "job", job.Name, "err", err)
	}
}

func (job *Job) Close() {
	job.mu.Lock()
	cancel := job.cancelFn
	closed := job.closed
	job.closed = true
	job.mu.Unlock()

	if cancel != nil {
		cancel()
	} else {
		if !closed {
			job.removeAll()
		}
	}
}

func (job *Job) refreshDiskuse() {
	var imgfiles []string
	var diskuse int64
	_ = filepath.WalkDir(job.Workdir, func(fpath string, d fs.DirEntry, err error) error {
		if err == nil {
			if fi, e := d.Info(); e == nil {
				diskuse += fi.Size()
			}
			if strings.HasSuffix(d.Name(), ".png") {
				imgfiles = append(imgfiles, d.Name())
			}
		}
		return nil
	})
	job.mu.Lock()
	job.diskuse = diskuse
	for _, fn := range imgfiles {
		if _, ok := job.imgfiles[fn]; !ok {
			job.imgfiles[fn] = false
		}
	}
	job.mu.Unlock()
	job.Jaws.Dirty(job, uiJobStatus{job})
}

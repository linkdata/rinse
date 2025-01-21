package rinser

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
	JobExtractMeta
	JobDetectLanguage
	JobDocToPdf
	JobPdfToImages
	JobTesseract
	JobEnding
	JobFinished
	JobFailed
)

type Job struct {
	Rinse         *Rinse         `json:"-"`
	Workdir       string         `json:"workdir" example:"/tmp/rinse-550e8400-e29b-41d4-a716-446655440000"`
	Datadir       string         `json:"-"`
	Name          string         `json:"name" example:"example.docx"`
	Created       time.Time      `json:"created" example:"2024-01-01T12:00:00+00:00" format:"dateTime"`
	UUID          uuid.UUID      `json:"uuid" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	MaxSizeMB     int            `json:"maxsizemb" example:"2048"`
	MaxTimeSec    int            `json:"maxtimesec" example:"86400"`
	CleanupSec    int            `json:"cleanupsec" example:"600"`
	TimeoutSec    int            `json:"timeoutsec" example:"600"`
	CleanupGotten bool           `json:"cleanupgotten" example:"true"`
	Private       bool           `json:"private" example:"false"`
	Email         string         `json:"email,omitempty" example:"user@example.com"`
	StoppedCh     chan struct{}  `json:"-"` // closed when job stopped
	mu            deadlock.Mutex // protects following
	Error         error          `json:"error,omitempty"`
	PdfName       string         `json:"pdfname,omitempty" example:"example-docx-rinsed.pdf"` // rinsed PDF file name
	Language      string         `json:"lang,omitempty" example:"auto"`
	Done          bool           `json:"done,omitempty" example:"false"`
	Diskuse       int64          `json:"diskuse,omitempty" example:"1234"`
	Pages         int            `json:"pages,omitempty" example:"1"`
	Downloads     int            `json:"downloads,omitempty" example:"0"`
	started       time.Time
	progress      time.Time // when we last saw progress being made
	stopped       time.Time
	docName       string // document file name, once known
	state         JobState
	imgfiles      map[string]bool
	cancelFn      context.CancelFunc
	closed        bool
	errstate      JobState
	previews      map[uint64][]byte
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

func NewJob(rns *Rinse, name, lang string, maxsizemb, maxtimesec, cleanupsec, timeoutsec int, cleanupgotten, private bool, email string) (job *Job, err error) {
	if err = checkLangString(lang); err == nil {
		if lang == "auto" {
			lang = ""
		}
		id := uuid.New()
		workDir := path.Join(os.TempDir(), "rinse-"+id.String())
		if err = os.Mkdir(workDir, 0777); err == nil /* #nosec G301 */ {
			dataDir := path.Join(workDir, "data")
			if err = os.Mkdir(dataDir, 0777); err == nil /* #nosec G301 */ {
				job = &Job{
					Rinse:         rns,
					Name:          name,
					Language:      lang,
					Workdir:       workDir,
					Datadir:       dataDir,
					Created:       time.Now(),
					UUID:          id,
					MaxSizeMB:     maxsizemb,
					MaxTimeSec:    maxtimesec,
					CleanupSec:    cleanupsec,
					TimeoutSec:    timeoutsec,
					CleanupGotten: cleanupgotten,
					Private:       private,
					Email:         email,
					state:         JobNew,
					StoppedCh:     make(chan struct{}),
					imgfiles:      make(map[string]bool),
					previews:      make(map[uint64][]byte),
				}
			}
		}
	}
	return
}

func (job *Job) HasMeta() (yes bool) {
	if job.State() > JobExtractMeta {
		if _, e := os.Stat(job.MetaPath()); e == nil {
			yes = true
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

func (job *Job) MaxUploadSize() (n int64) {
	n = int64(job.MaxSizeMB) * 1024 * 1024
	return
}

func (job *Job) Start() (err error) {
	if err = job.transition(JobNew, JobStarting); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(job.MaxTimeSec)*time.Second)
		job.mu.Lock()
		job.cancelFn = cancel
		job.mu.Unlock()
		go job.watchProgress(ctx)
		go job.process(ctx)
		if l := job.Rinse.Config.Logger; l != nil {
			job.Rinse.Config.Logger.Info("job started", "job", job.Name, "email", job.Email)
		}
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

func (job *Job) Stopped() (t time.Time) {
	job.mu.Lock()
	t = job.stopped
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

func (job *Job) MetaPath() string {
	return path.Join(job.Datadir, job.DocumentName()+".json")
}

func (job *Job) ResultPath() string {
	return path.Join(job.Datadir, job.ResultName())
}

func (job *Job) transition(fromState, toState JobState) (err error) {
	job.mu.Lock()
	if job.state == fromState {
		job.state = toState
		job.progress = time.Now()
	} else {
		err = fmt.Errorf("expected job state %d, have %d", fromState, job.state)
	}
	job.mu.Unlock()
	job.refreshDiskuse()
	return
}

func (job *Job) madeProgress() {
	job.mu.Lock()
	job.progress = time.Now()
	job.mu.Unlock()
}

func (job *Job) runsc(ctx context.Context, stdouthandler func(string, bool) error, cmds ...string) (err error) {
	defer job.madeProgress()
	return runsc(ctx, job.Rinse.RunscBin, job.Rinse.RootDir, job.Workdir, job.UUID.String(), stdouthandler, cmds...)
}

func (job *Job) removeAll() {
	if err := scrub(job.Workdir); err != nil {
		slog.Error("job.removeAll", "job", job.Name, "err", err)
	}
}

func (job *Job) Close(err error) {
	job.mu.Lock()
	cancel := job.cancelFn
	closed := job.closed
	job.closed = true
	if job.Error == nil {
		job.Error = err
	}
	job.mu.Unlock()

	if cancel != nil {
		cancel()
	} else {
		if !closed {
			job.removeAll()
		}
	}
}

func (job *Job) getImageFiles() (fns []string) {
	job.mu.Lock()
	defer job.mu.Unlock()
	for fn, ok := range job.imgfiles {
		if ok {
			fns = append(fns, fn)
		}
	}
	return
}

func (job *Job) refreshDiskuse() {
	var imgfiles []string
	var diskuse int64
	_ = filepath.WalkDir(job.Datadir, func(fpath string, d fs.DirEntry, err error) error {
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
	now := time.Now()
	job.mu.Lock()
	job.Diskuse = diskuse
	for _, fn := range imgfiles {
		if _, ok := job.imgfiles[fn]; !ok {
			job.imgfiles[fn] = false
			job.progress = now
		}
	}
	job.mu.Unlock()
	job.Rinse.Jaws.Dirty(job, uiJobStatus{job})
}

func (job *Job) downloaded() {
	job.mu.Lock()
	job.Downloads++
	job.mu.Unlock()
	if job.CleanupGotten {
		job.Rinse.RemoveJob(job)
	}
}

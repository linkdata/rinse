package rinse

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
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
	JobPdfToPPm
	JobTesseract
	JobFinished
	JobFailed
)

type Job struct {
	*Rinse
	Name     string
	Lang     string
	Workdir  string
	Created  time.Time
	UUID     uuid.UUID
	mu       deadlock.Mutex
	state    JobState
	resultCh chan error
	started  time.Time
	stopped  time.Time
	closed   bool
	ppmtodo  []string
	ppmdone  []string
	diskuse  int64
}

var ErrNotPDF = errors.New("input file must be a PDF")
var ErrIllegalLanguage = errors.New("illegal language string")

func checkExt(name string) error {
	if strings.ToLower(filepath.Ext(name)) != ".pdf" {
		return ErrNotPDF
	}
	return nil
}

func checkLangString(lang string) error {
	for _, ch := range lang {
		if !(ch == '+' || (ch >= 'a' && ch <= 'z')) {
			return ErrIllegalLanguage
		}
	}
	return nil
}

func NewJob(rns *Rinse, name, lang string) (job *Job, err error) {
	if err = checkExt(name); err == nil {
		if err = checkLangString(lang); err == nil {
			var workdir string
			if workdir, err = os.MkdirTemp("", "rinse-"); err == nil {
				job = &Job{
					Rinse:    rns,
					Name:     filepath.Base(name),
					Lang:     lang,
					Workdir:  workdir,
					Created:  time.Now(),
					UUID:     uuid.New(),
					state:    JobNew,
					resultCh: make(chan error, 1),
				}
			}
		}
	}
	return
}

func (job *Job) renameInput() (err error) {
	if job.Name != "input.pdf" {
		if err = os.WriteFile(path.Join(job.Workdir, "input.txt"), []byte(job.Name), 0666); err == nil {
			err = os.Rename(path.Join(job.Workdir, job.Name), path.Join(job.Workdir, "input.pdf"))
		}
	}
	return
}

func (job *Job) Start() (err error) {
	if err = job.transition(JobNew, JobStarting); err == nil {
		if err = job.renameInput(); err == nil {
			job.mu.Lock()
			job.started = time.Now()
			job.mu.Unlock()
			go job.runPdfToPpm()
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

func (job *Job) checkErrLocked(err error) {
	if err != nil && job.state != JobFailed {
		job.state = JobFailed
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
	job.Jaws.Dirty(job, uiJobPagecount{job})
	return
}

func (job *Job) waitForPdfToPpm(cmd *exec.Cmd) (err error) {
	var done int32
	defer atomic.StoreInt32(&done, 1)
	go func() {
		for atomic.LoadInt32(&done) == 0 {
			time.Sleep(time.Millisecond * 100)
			job.Jaws.Dirty(job, uiJobPagecount{job})
		}
	}()
	var output []byte
	output, err = cmd.CombinedOutput()
	output = bytes.TrimSpace(output)
	if len(output) > 0 {
		slog.Warn("rinse-pdftoppm", "msg", string(output))
	}
	return
}

func (job *Job) runPdfToPpm() {
	err := job.transition(JobStarting, JobPdfToPPm)

	if err == nil {
		var args []string
		if job.RunscBin != "" {
			args = append(args, "--runtime="+job.RunscBin)
		}
		args = append(args, "--log-level=error", "run", "--rm",
			"--userns=keep-id:uid=1000,gid=1000",
			"-v", job.Workdir+":/var/rinse", PodmanImage,
			"pdftoppm", "-cropbox", "/var/rinse/input.pdf", "/var/rinse/output")
		cmd := exec.Command(job.PodmanBin, args...)
		if err = job.waitForPdfToPpm(cmd); err == nil {
			if err = os.Remove(path.Join(job.Workdir, "input.pdf")); err == nil {
				var outputFiles []string
				filepath.WalkDir(job.Workdir, func(fpath string, d fs.DirEntry, err error) error {
					if d.Type().IsRegular() && filepath.Ext(fpath) == ".ppm" {
						outputFiles = append(outputFiles, d.Name())
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
						job.Jaws.Dirty(job, uiJobPagecount{job})
						job.mu.Unlock()
						if err = job.runTesseract(); err == nil {
							if err = job.transition(JobTesseract, JobFinished); err == nil {
								job.mu.Lock()
								job.stopped = time.Now()
								ch := job.resultCh
								job.Jaws.Dirty(job, uiJobPagecount{job})
								job.mu.Unlock()
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
		args = append(args, "--log-level=error", "run", "--rm", "--tty",
			"--env", fmt.Sprintf("OMP_THREAD_LIMIT=%d", runtime.NumCPU()),
			"--userns=keep-id:uid=1000,gid=1000",
			"-v", job.Workdir+":/var/rinse", PodmanImage,
			"tesseract")
		if job.Lang != "" {
			args = append(args, "-l", job.Lang)
		}
		args = append(args, "/var/rinse/output.txt", "/var/rinse/output", "pdf")
		cmd := exec.Command(job.PodmanBin, args...)
		var stdout io.ReadCloser
		if stdout, err = cmd.StdoutPipe(); err == nil {
			if err = cmd.Start(); err == nil {
				lineScanner := bufio.NewScanner(stdout)
				for lineScanner.Scan() {
					s := lineScanner.Text()
					job.mu.Lock()
					job.ppmtodo = slices.DeleteFunc(job.ppmtodo, func(fn string) bool {
						if strings.Contains(s, fn) {
							job.ppmdone = append(job.ppmdone, fn)
							return true
						}
						return false
					})
					job.mu.Unlock()
					job.Jaws.Dirty(job, uiJobPagecount{job})
				}
				if err = cmd.Wait(); err == nil {
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
					job.mu.Lock()
					defer job.mu.Unlock()
					job.diskuse, _ = job.getDiskuse()
					job.Jaws.Dirty(job, uiJobPagecount{job})
				}
			}
		}
	}
	return
}

func (job *Job) Result() (err error) {
	if err = os.Rename(path.Join(job.Workdir, "output.pdf"), path.Join(job.Workdir, job.Name)); err == nil {
		_ = os.Remove(path.Join(job.Workdir, "input.txt"))
	}
	return
}

func (job *Job) Close() (err error) {
	job.mu.Lock()
	defer job.mu.Unlock()
	if !job.closed {
		job.closed = true
		if job.state != JobFinished {
			job.state = JobFailed
		}
		close(job.resultCh)
		err = os.RemoveAll(job.Workdir)
	}
	return
}

func (job *Job) getDiskuse() (diskuse int64, nfiles int) {
	filepath.WalkDir(job.Workdir, func(path string, d fs.DirEntry, err error) error {
		if fi, err := d.Info(); err == nil {
			diskuse += fi.Size()
		}
		if filepath.Ext(d.Name()) == ".ppm" {
			nfiles++
		}
		return nil
	})
	return
}

package rinse

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

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
	Name      string
	PodmanBin string
	RunscBin  string
	Workdir   string
	mu        deadlock.Mutex
	state     JobState
	resultCh  chan error
	started   time.Time
	stopped   time.Time
	closed    bool
	ppmfiles  []string
}

func NewJob(name, podmanbin, runscbin string) (job *Job, err error) {
	var workdir string
	if workdir, err = os.MkdirTemp("", "rinse-"); err == nil {
		job = &Job{
			Name:      filepath.Base(name),
			PodmanBin: podmanbin,
			RunscBin:  runscbin,
			Workdir:   workdir,
			state:     JobNew,
			resultCh:  make(chan error, 1),
		}
	}
	return
}

// podman --runtime=/usr/bin/runsc run --rm -v $DIR_WITH_INPUT_PDF:/var/rinse -it ghcr.io/linkdata/rinse-pdftoppm:latest
// podman run --rm -v $DIR_WITH_OUTPUT_PPM:/var/rinse -it ghcr.io/linkdata/rinse-tesseract:latest

func (job *Job) Start() (err error) {
	if err = job.transition(JobNew, JobStarting); err == nil {
		if job.Name != "input.pdf" {
			err = os.Rename(path.Join(job.Workdir, job.Name), path.Join(job.Workdir, "input.pdf"))
		}
		if err == nil {
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
	if err != nil {
		job.state = JobFailed
		job.resultCh <- err
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
		err = fmt.Errorf("wrong job state (%d)", job.state)
	}
	job.mu.Unlock()
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
			"-v", job.Workdir+":/var/rinse", "ghcr.io/linkdata/rinse:latest",
			"pdftoppm", "-cropbox", "/var/rinse/input.pdf", "/var/rinse/output")
		cmd := exec.Command(job.PodmanBin, args...)
		// we expect no output from pdftoppm
		var output []byte
		output, err = cmd.CombinedOutput()
		output = bytes.TrimSpace(output)
		if len(output) > 0 {
			slog.Warn("rinse-pdftoppm", "msg", string(output))
		}
		if err == nil {
			if err = os.Remove(path.Join(job.Workdir, "input.pdf")); err == nil {
				var outputFiles []string
				filepath.Walk(job.Workdir, func(fpath string, info fs.FileInfo, err error) error {
					if filepath.Ext(fpath) == ".ppm" {
						outputFiles = append(outputFiles, info.Name())
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
						job.ppmfiles = outputFiles
						job.mu.Unlock()
						if err = job.runTesseract(); err == nil {
							if err = job.transition(JobTesseract, JobFinished); err == nil {
								job.mu.Lock()
								job.stopped = time.Now()
								ch := job.resultCh
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
			"--userns=keep-id:uid=1000,gid=1000",
			"-v", job.Workdir+":/var/rinse", "ghcr.io/linkdata/rinse:latest",
			"tesseract", "/var/rinse/output.txt", "/var/rinse/output", "pdf")
		cmd := exec.Command(job.PodmanBin, args...)
		var stdout io.ReadCloser
		if stdout, err = cmd.StdoutPipe(); err == nil {
			if err = cmd.Start(); err == nil {
				lineScanner := bufio.NewScanner(stdout)
				lineScanner.Split(bufio.ScanLines)
				for lineScanner.Scan() {
					s := lineScanner.Text()
					job.mu.Lock()
					job.ppmfiles = slices.DeleteFunc(job.ppmfiles, func(fn string) bool { return strings.Contains(s, fn) })
					job.mu.Unlock()
				}
				err = cmd.Wait()
			}
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

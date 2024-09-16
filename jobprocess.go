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
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

var ErrPpmSeenTwice = errors.New("ppm file seen twice")

func (job *Job) process(ctx context.Context) {
	job.mu.Lock()
	job.started = time.Now()
	job.mu.Unlock()
	defer job.processDone()
	fn, err := job.renameInput()
	if err == nil {
		if err = job.runDetectLanguage(ctx, fn); err == nil {
			if err = job.runDocToPdf(ctx, fn); err == nil {
				if err = job.runPdfToPpm(ctx); err == nil {
					if err = job.runTesseract(ctx); err == nil {
						if err = job.jobEnding(); err == nil {
							if err = job.transition(JobEnding, JobFinished); err == nil {
								return
							}
						}
					}
				}
			}
		}
	}
	if !errors.Is(err, context.Canceled) {
		slog.Error("job failed", "job", job.Name, "state", job.State(), "err", err)
	}
	job.mu.Lock()
	job.state = JobFailed
	job.err = err
	job.mu.Unlock()
}

func (job *Job) processDone() {
	job.mu.Lock()
	job.stopped = time.Now()
	job.cancelFn = nil
	closed := job.closed
	job.mu.Unlock()
	if closed {
		job.removeAll()
	} else {
		job.cleanup("")
	}
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

func (job *Job) runDetectLanguage(ctx context.Context, fn string) (err error) {
	// java -jar /usr/local/bin/tika.jar --language /var/rinse/input.ext 2>/dev/null
	if err = job.transition(JobStarting, JobDetect); err == nil {
		if job.Lang == "auto" {
			lang := "eng"
			stdouthandler := func(s string) (err error) {
				if len(s) == 2 {
					lang = s
				}
				return
			}
			if e := job.podrun(ctx, stdouthandler, "java", "-jar", "/usr/local/bin/tika.jar", "--language", "/var/rinse/"+fn); e == nil {
				if s, ok := LanguageTika[lang]; ok {
					lang = s
				}
			}
			job.mu.Lock()
			job.Lang = lang
			job.mu.Unlock()
		}
	}
	return
}

func (job *Job) waitForDocToPdf(ctx context.Context, fn string) (err error) {
	if !strings.HasSuffix(fn, ".pdf") {
		if err = job.podrun(ctx, nil, "libreoffice", "--headless", "--safe-mode", "--convert-to", "pdf", "--outdir", "/var/rinse", "/var/rinse/"+fn); err == nil {
			err = os.Remove(path.Join(job.Workdir, fn))
		}
	}
	return
}

func (job *Job) runDocToPdf(ctx context.Context, fn string) (err error) {
	if err = job.transition(JobDetect, JobDocToPdf); err == nil {
		err = job.waitForDocToPdf(ctx, fn)
	}
	return
}

func (job *Job) waitForPdfToPpm(ctx context.Context) (err error) {
	var done int32
	defer atomic.StoreInt32(&done, 1)
	go func() {
		for atomic.LoadInt32(&done) == 0 {
			time.Sleep(time.Millisecond * 500)
			job.refreshDiskuse()
		}
	}()
	return job.podrun(ctx, nil, "pdftoppm", "-cropbox", "/var/rinse/input.pdf", "/var/rinse/output")
}

func (job *Job) makeOutputTxt() (err error) {
	var f *os.File
	if f, err = os.OpenFile(path.Join(job.Workdir, "output.txt"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666); err == nil {
		defer f.Close()
		job.mu.Lock()
		var outputFiles []string
		for fn := range job.ppmfiles {
			outputFiles = append(outputFiles, fn)
		}
		job.mu.Unlock()
		sort.Strings(outputFiles)
		for _, fn := range outputFiles {
			if _, err = fmt.Fprintf(f, "/var/rinse/%s\n", fn); err != nil {
				return
			}
		}
		err = f.Sync()
	}
	return
}

func (job *Job) runPdfToPpm(ctx context.Context) (err error) {
	if err = job.transition(JobDocToPdf, JobPdfToPPm); err == nil {
		if err = job.waitForPdfToPpm(ctx); err == nil {
			if err = os.Remove(path.Join(job.Workdir, "input.pdf")); err == nil {
				job.refreshDiskuse()
				err = job.makeOutputTxt()
			}
		}
	}
	return
}

func (job *Job) runTesseract(ctx context.Context) (err error) {
	if err = job.transition(JobPdfToPPm, JobTesseract); err == nil {
		var output []string
		stdouthandler := func(s string) error {
			defer job.Jaws.Dirty(uiJobStatus{job})
			job.mu.Lock()
			defer job.mu.Unlock()
			output = append(output, s)
			for fn, seen := range job.ppmfiles {
				if strings.Contains(s, fn) {
					if seen {
						if strings.Contains(s, "file not found") {
							return errors.New(s)
						}
						return ErrPpmSeenTwice
					}
					job.ppmfiles[fn] = true
					break
				}
			}
			return nil
		}
		if err = job.podrun(ctx, stdouthandler, "tesseract", "-l", job.Lang, "/var/rinse/output.txt", "/var/rinse/output", "pdf"); err != nil {
			if !(errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
				for _, s := range output {
					slog.Error("tesseract", "msg", s)
				}
			}
		}
	}
	return
}

func (job *Job) cleanup(except string) (err error) {
	var diskuse int64
	err = filepath.WalkDir(job.Workdir, func(fpath string, d fs.DirEntry, err error) error {
		if err == nil {
			if d.Type().IsRegular() {
				if except == "" || except != d.Name() {
					_ = os.Remove(fpath)
				} else {
					if fi, e := d.Info(); e == nil {
						diskuse += fi.Size()
					}
				}
			}
		}
		return nil
	})
	job.mu.Lock()
	job.diskuse = diskuse
	job.mu.Unlock()
	job.Jaws.Dirty(job, uiJobStatus{job})
	return
}

func (job *Job) jobEnding() (err error) {
	if err = job.transition(JobTesseract, JobEnding); err == nil {
		if err = job.cleanup("output.pdf"); err == nil {
			err = os.Rename(path.Join(job.Workdir, "output.pdf"), path.Join(job.Workdir, job.ResultName))
		}
	}
	return
}

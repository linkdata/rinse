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

	var err error
	if err = job.runDownload(ctx); err == nil {
		var wrkName string
		if wrkName, err = job.runDocumentName(); err == nil {
			if err = job.runDetectLanguage(ctx, wrkName); err == nil {
				if err = job.runDocToPdf(ctx, wrkName); err == nil {
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
	}

	if !errors.Is(err, context.Canceled) {
		slog.Error("job failed", "job", job.Name, "state", jobStateText(job.State()), "err", err)
	}
	job.mu.Lock()
	job.errstate = job.state
	job.err = err
	job.state = JobFailed
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
		_ = job.cleanup(job.ResultName())
	}
}

var ErrIllegalURLScheme = errors.New("illegal URL scheme")
var ErrMultipleDocuments = errors.New("multiple documents found")
var ErrMissingDocument = errors.New("no document found")

func hasHTTPScheme(s string) bool {
	return strings.HasPrefix(s, "http:") || strings.HasPrefix(s, "https:")
}

func (job *Job) runDownload(ctx context.Context) (err error) {
	if err = job.transition(JobStarting, JobDownload); err == nil {
		if hasHTTPScheme(job.Name) {
			var msgs []string
			stdouthandler := func(s string) (err error) {
				msgs = append(msgs, s)
				return
			}
			args := []string{
				"wget",
				"--quiet",
				"--content-disposition",
				"--no-directories",
				"--directory-prefix=/var/rinse",
			}
			if n := job.MaxUploadSize(); n > 0 {
				args = append(args, fmt.Sprintf("--quota=%v", job.MaxUploadSize()))
			}
			args = append(args, job.Name)
			if err = job.podrun(ctx, stdouthandler, args...); err != nil {
				for _, s := range msgs {
					slog.Error("wget", "msg", s)
				}
			}
		}
	}
	return
}

func mustHaveDocument(s string) error {
	if s == "" {
		return ErrMissingDocument
	}
	return nil
}

func (job *Job) runDocumentName() (wrkName string, err error) {
	var docName string
	err = filepath.WalkDir(job.Workdir, func(fpath string, d fs.DirEntry, err error) error {
		if err == nil {
			if d.Type().IsRegular() {
				if docName != "" {
					slog.Error("more than one document", "docName", docName, "other", d.Name())
					return ErrMultipleDocuments
				}
				docName = d.Name()
			}
		}
		return nil
	})

	if err == nil {
		if err = mustHaveDocument(docName); err == nil {
			ext := filepath.Ext(docName)

			job.mu.Lock()
			job.docName = docName
			job.pdfName = strings.ReplaceAll(strings.TrimSuffix(docName, ext)+"-rinsed.pdf", "\"", "")
			job.mu.Unlock()

			wrkName = "input" + strings.ToLower(ext)
			src := path.Join(job.Workdir, docName)
			dst := path.Join(job.Workdir, wrkName)
			if err = os.Rename(src, dst); err == nil {
				err = os.Chmod(dst, 0444) // #nosec G302
			}
		}
	}
	return
}

func (job *Job) runDetectLanguage(ctx context.Context, fn string) (err error) {
	if err = job.transition(JobDownload, JobDetectLanguage); err == nil {
		if job.Lang() == "" {
			var lang string
			stdouthandler := func(s string) (err error) {
				if len(s) == 2 {
					if l, ok := LanguageTika[s]; ok {
						lang = l
					}
				}
				return
			}
			if e := job.podrun(ctx, stdouthandler, "java", "-jar", "/usr/local/bin/tika.jar", "--language", "/var/rinse/"+fn); e == nil {
				job.mu.Lock()
				job.lang = lang
				job.mu.Unlock()
			}
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
	if err = job.transition(JobDetectLanguage, JobDocToPdf); err == nil {
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
	fpath := filepath.Clean(path.Join(job.Workdir, "output.txt"))
	if f, err = os.OpenFile(fpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644); err == nil { // #nosec G302
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
		args := []string{
			"tesseract",
		}
		if s := job.Lang(); s != "" {
			args = append(args, "-l", s)
		}
		args = append(args, "/var/rinse/output.txt", "/var/rinse/output", "pdf")
		if err = job.podrun(ctx, stdouthandler, args...); err != nil {
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
			err = os.Rename(path.Join(job.Workdir, "output.pdf"), path.Join(job.Workdir, job.ResultName()))
		}
	}
	return
}

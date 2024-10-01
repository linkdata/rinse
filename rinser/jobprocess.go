package rinser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

var ErrImageSeenTwice = errors.New("image file seen twice")

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
					if err = job.runPdfToImages(ctx); err == nil {
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
	job.Error = err
	job.state = JobFailed
	job.mu.Unlock()
	job.Jaws.Dirty(uiJobStatus{job})
}

func (job *Job) processDone() {
	job.mu.Lock()
	job.stopped = time.Now()
	job.Done = true
	job.cancelFn = nil
	closed := job.closed
	job.mu.Unlock()
	if closed {
		job.removeAll()
	}
}

var ErrIllegalURLScheme = errors.New("illegal URL scheme")
var ErrMultipleDocuments = errors.New("multiple documents found")
var ErrMissingDocument = errors.New("no document found")
var ErrDocumentTooLarge = errors.New("document too large")

func hasHTTPScheme(s string) bool {
	return strings.HasPrefix(s, "http:") || strings.HasPrefix(s, "https:")
}

func (job *Job) limitDocumentSize(resp *http.Response) (src io.Reader, maxUploadSize int64, err error) {
	src = resp.Body
	if maxUploadSize = job.MaxUploadSize(); maxUploadSize > 0 {
		src = io.LimitReader(src, maxUploadSize+1)
		if resp.ContentLength > 0 && resp.ContentLength > maxUploadSize {
			err = ErrDocumentTooLarge
		}
	}
	return
}

func (job *Job) runDownload(ctx context.Context) (err error) {
	if err = job.transition(JobStarting, JobDownload); err == nil {
		if hasHTTPScheme(job.Name) {
			var req *http.Request
			if req, err = http.NewRequestWithContext(ctx, http.MethodGet, job.Name, nil); err == nil {
				var resp *http.Response
				if resp, err = http.DefaultClient.Do(req); err == nil { // #nosec G107
					srcName := resp.Request.URL.Path
					if cd := resp.Header.Get("Content-Disposition"); cd != "" {
						if _, params, e := mime.ParseMediaType(cd); e == nil {
							if s, ok := params["filename"]; ok {
								srcName = s
							}
						}
					}
					srcName = path.Base(srcName)
					if filepath.Ext(srcName) == "" {
						if ct := resp.Header.Get("Content-Type"); ct != "" {
							if mediatype, _, e := mime.ParseMediaType(ct); e == nil {
								if exts, e := mime.ExtensionsByType(mediatype); e == nil {
									srcName += exts[0]
								}
							}
						}
					}

					var srcFile io.Reader
					var maxUploadSize int64
					if srcFile, maxUploadSize, err = job.limitDocumentSize(resp); err == nil {
						var of *os.File
						if of, err = os.Create(path.Join(job.Datadir, srcName)); err == nil {
							defer of.Close()
							var written int64
							if written, err = io.Copy(of, srcFile); err == nil {
								if maxUploadSize < 1 || written <= maxUploadSize {
									if err = of.Close(); err == nil {
										return
									}
								}
								err = ErrDocumentTooLarge
							}
						}
					}
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
	err = filepath.WalkDir(job.Datadir, func(fpath string, d fs.DirEntry, err error) error {
		if err == nil {
			if d.Type().IsRegular() {
				if docName != "" {
					slog.Error("more than one document", "docName", docName, "other", d.Name())
					return ErrMultipleDocuments
				}
				if d.Name() == ".wget-hsts" {
					_ = scrub(fpath)
				} else {
					docName = d.Name()
				}
			}
		}
		return nil
	})

	if err == nil {
		if err = mustHaveDocument(docName); err == nil {
			ext := filepath.Ext(docName)

			job.mu.Lock()
			job.docName = docName
			job.PdfName = strings.ReplaceAll(strings.TrimSuffix(docName, ext)+"-rinsed.pdf", "\"", "")
			job.mu.Unlock()

			wrkName = "input" + strings.ToLower(ext)
			src := path.Join(job.Datadir, docName)
			dst := path.Join(job.Datadir, wrkName)
			if err = os.Rename(src, dst); err == nil {
				err = os.Chmod(dst, 0644) // #nosec G302
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
				job.Language = lang
				job.mu.Unlock()
			}
		}
	}
	return
}

func (job *Job) waitForDocToPdf(ctx context.Context, fn string) (err error) {
	if !strings.HasSuffix(fn, ".pdf") {
		if err = job.podrun(ctx, nil, "libreoffice", "--headless", "--safe-mode", "--convert-to", "pdf", "--outdir", "/var/rinse", "/var/rinse/"+fn); err == nil {
			err = scrub(path.Join(job.Datadir, fn))
		}
	}
	return
}

func (job *Job) runDocToPdf(ctx context.Context, fn string) (err error) {
	if err = job.transition(JobDetectLanguage, JobDocToPdf); err == nil {
		if err = job.waitForDocToPdf(ctx, fn); err == nil {
			if err = scrub(path.Join(job.Datadir, ".cache")); err == nil {
				err = scrub(path.Join(job.Datadir, ".config"))
			}
		}
	}
	return
}

func (job *Job) waitForPdfToImages(ctx context.Context) (err error) {
	var done int32
	defer atomic.StoreInt32(&done, 1)
	go func() {
		for atomic.LoadInt32(&done) == 0 {
			time.Sleep(time.Millisecond * 500)
			job.refreshDiskuse()
		}
	}()
	return job.podrun(ctx, nil, "pdftoppm", "-png", "-cropbox", "/var/rinse/input.pdf", "/var/rinse/output")
}

func (job *Job) makeOutputTxt() (err error) {
	var f *os.File
	fpath := filepath.Clean(path.Join(job.Datadir, "output.txt"))
	if f, err = os.OpenFile(fpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644); err == nil { // #nosec G302
		defer f.Close()
		job.mu.Lock()
		var outputFiles []string
		for fn := range job.imgfiles {
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

func (job *Job) runPdfToImages(ctx context.Context) (err error) {
	if err = job.transition(JobDocToPdf, JobPdfToImages); err == nil {
		if err = job.waitForPdfToImages(ctx); err == nil {
			if err = scrub(path.Join(job.Datadir, "input.pdf")); err == nil {
				job.refreshDiskuse()
				err = job.makeOutputTxt()
			}
		}
	}
	return
}

func (job *Job) runTesseract(ctx context.Context) (err error) {
	if err = job.transition(JobPdfToImages, JobTesseract); err == nil {
		var output []string
		stdouthandler := func(s string) error {
			defer job.Jaws.Dirty(uiJobStatus{job})
			job.mu.Lock()
			defer job.mu.Unlock()
			output = append(output, s)
			for fn, seen := range job.imgfiles {
				if strings.Contains(s, fn) {
					if seen {
						if strings.Contains(s, "file not found") {
							return errors.New(s)
						}
						return ErrImageSeenTwice
					}
					job.imgfiles[fn] = true
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

func (job *Job) jobEnding() (err error) {
	if err = job.transition(JobTesseract, JobEnding); err == nil {
		if err = os.Rename(path.Join(job.Datadir, "output.pdf"), path.Join(job.Datadir, job.ResultName())); err == nil {
			var diskuse int64
			err = filepath.WalkDir(job.Datadir, func(fpath string, d fs.DirEntry, err error) error {
				if err == nil {
					if d.Type().IsRegular() {
						switch filepath.Ext(d.Name()) {
						case ".png", ".pdf":
							if fi, e := d.Info(); e == nil {
								diskuse += fi.Size()
							}
						default:
							_ = scrub(fpath)
						}
					}
				}
				return nil
			})
			job.mu.Lock()
			job.Diskuse = diskuse
			job.mu.Unlock()
			job.Jaws.Dirty(job, uiJobStatus{job})
		}
	}
	return
}

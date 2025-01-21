package rinser

import (
	"bytes"
	"context"
	"encoding/json"
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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var ErrImageSeenTwice = errors.New("image file seen twice")

func (job *Job) watchProgress(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-job.StoppedCh:
			return
		case <-ticker.C:
			job.mu.Lock()
			progress := job.progress
			timeout := time.Duration(job.TimeoutSec) * time.Second
			job.mu.Unlock()

			if !progress.IsZero() && timeout > 0 {
				if time.Since(progress) > timeout {
					job.Close(fmt.Errorf("no progress made for %v", timeout))
				}
			}
		}
	}
}

func (job *Job) process(ctx context.Context) {
	now := time.Now()
	job.mu.Lock()
	job.started = now
	job.progress = now
	job.mu.Unlock()
	defer job.processDone()

	var err error
	if err = job.runDownload(ctx); err == nil {
		var docName, wrkName string
		if docName, wrkName, err = job.runDocumentName(); err == nil {
			if err = job.runExtractMeta(ctx, docName); err == nil {
				if err = job.renameDoc(docName, wrkName); err == nil {
					if err = job.runDetectLanguage(ctx, wrkName); err == nil {
						if err = job.runDocToPdf(ctx, wrkName); err == nil {
							if err = job.runPdfToImages(ctx); err == nil {
								if err = job.runTesseract(ctx); err == nil {
									if err = job.jobEnding(ctx); err == nil {
										if err = job.transition(ctx, JobEnding, JobFinished); err == nil {
											return
										}
									}
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
	if job.Error == nil {
		job.Error = err
	}
	job.state = JobFailed
	job.mu.Unlock()
	job.Rinse.Jaws.Dirty(uiJobStatus{job})
}

func (job *Job) processDone() {
	job.mu.Lock()
	job.stopped = time.Now()
	job.Done = true
	job.cancelFn = nil
	closed := job.closed
	close(job.StoppedCh)
	job.mu.Unlock()
	if l := job.Rinse.Config.Logger; l != nil {
		job.Rinse.Config.Logger.Info("job stopped", "job", job.Name, "email", job.Email)
	}
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
	if err = job.transition(ctx, JobStarting, JobDownload); err == nil {
		if hasHTTPScheme(job.Name) {
			var req *http.Request
			if req, err = http.NewRequestWithContext(ctx, http.MethodGet, job.Name, nil); err == nil {
				var resp *http.Response
				if resp, err = job.Rinse.getClient().Do(req); err == nil { // #nosec G107
					if resp.StatusCode == http.StatusOK {
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
							if of, err = os.Create(path.Join(job.Datadir, srcName)); err == nil /* #nosec G304 */ {
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
					} else {
						err = errors.New(resp.Status)
					}
				}
			}
		}
	}
	return
}

func mustHaveDocument(s string, n int64) error {
	if s == "" || n < 1 {
		return ErrMissingDocument
	}
	return nil
}

func (job *Job) runDocumentName() (docName, wrkName string, err error) {
	var docSize int64
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
					if fi, e := d.Info(); e == nil {
						docSize = fi.Size()
					}
				}
			}
		}
		return nil
	})

	if err == nil {
		if err = mustHaveDocument(docName, docSize); err == nil {
			ext := filepath.Ext(docName)

			job.mu.Lock()
			job.docName = docName
			job.PdfName = strings.ReplaceAll(strings.TrimSuffix(docName, ext)+"-"+strings.TrimPrefix(ext, ".")+"-rinsed.pdf", "\"", "")
			job.mu.Unlock()

			wrkName = "input" + strings.ToLower(ext)
		}
	}
	return
}

func (job *Job) renameDoc(docName, wrkName string) (err error) {
	src := path.Join(job.Datadir, docName)
	dst := path.Join(job.Datadir, wrkName)
	if err = os.Rename(src, dst); err == nil {
		err = os.Chmod(dst, 0644) // #nosec G302
	}
	return
}

func (job *Job) runExtractMeta(ctx context.Context, docName string) (err error) {
	if err = job.transition(ctx, JobDownload, JobExtractMeta); err == nil {
		var buf bytes.Buffer
		var errlines []string
		stdouthandler := func(s string, isout bool) (err error) {
			if isout {
				buf.WriteString(s)
			} else {
				errlines = append(errlines, s)
			}
			return
		}
		if e := job.runsc(ctx, stdouthandler, "java", "-jar", "/usr/local/bin/tika.jar", "--config=/tika-config.xml", "--json", "/var/rinse/"+docName); e == nil {
			var obj any
			if err = json.Unmarshal(buf.Bytes(), &obj); err == nil {
				fpath := filepath.Clean(path.Join(job.Datadir, job.docName+".json"))
				var b []byte
				if b, err = json.MarshalIndent(obj, "", "  "); err == nil {
					err = os.WriteFile(fpath, b, 0644) // #nosec G306
				}
			}
		}
		for _, s := range errlines {
			if !strings.HasPrefix(s, "DEBUG") && !strings.HasPrefix(s, "WARN") {
				slog.Error("metaextract", "job", job.Name, "stderr", s)
			}
		}
	}
	return
}

var detectLanguageRx = regexp.MustCompile(`DetectedLanguage\[(\w+):(\d\.\d+)\]`)

func (job *Job) runDetectLanguage(ctx context.Context, fn string) (err error) {
	if err = job.transition(ctx, JobExtractMeta, JobDetectLanguage); err == nil {
		if job.Lang() == "" {
			langs := map[string]struct{}{}
			stdouthandler := func(s string, isout bool) (err error) {
				job.madeProgress()
				if matches := detectLanguageRx.FindAllStringSubmatch(s, -1); matches != nil {
					for _, match := range matches {
						if len(match) > 1 {
							if l, ok := LanguageTika[match[1]]; ok {
								if confidence, err := strconv.ParseFloat(match[2], 64); err == nil {
									if confidence > 0.99 {
										if _, ok := langs[l]; !ok {
											langs[l] = struct{}{}
											slog.Info("detectLanguage", "job", job.Name, "lang", l, "confidence", confidence)
										}
									}
								}
							}
						}
					}
				}
				return
			}
			if e := job.runsc(ctx, stdouthandler, "java", "-jar", "/usr/local/bin/tika.jar", "-v", "--language", "/var/rinse/"+fn); e == nil {
				var languages []string
				for l := range langs {
					languages = append(languages, l)
				}
				job.mu.Lock()
				job.Language = strings.Join(languages, "+")
				job.mu.Unlock()
			}
		}
	}
	return
}

func (job *Job) waitForDocToPdf(ctx context.Context, fn string) (err error) {
	if !strings.HasSuffix(fn, ".pdf") {
		if err = job.runsc(ctx, nil, "libreoffice", "--headless", "--safe-mode", "--convert-to", "pdf", "--outdir", "/var/rinse", "/var/rinse/"+fn); err == nil {
			err = scrub(path.Join(job.Datadir, fn))
		}
	}
	return
}

func (job *Job) runDocToPdf(ctx context.Context, fn string) (err error) {
	if err = job.transition(ctx, JobDetectLanguage, JobDocToPdf); err == nil {
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
	return job.runsc(ctx, nil, "pdftoppm", "-png", "-cropbox", "/var/rinse/input.pdf", "/var/rinse/output")
}

func (job *Job) makeOutputTxt() (err error) {
	var f *os.File
	fpath := filepath.Clean(path.Join(job.Datadir, "output.txt"))
	if f, err = os.OpenFile(fpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644); err == nil /* #nosec G302 */ {
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
	if err = job.transition(ctx, JobDocToPdf, JobPdfToImages); err == nil {
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
	if err = job.transition(ctx, JobPdfToImages, JobTesseract); err == nil {
		var output []string
		stdouthandler := func(s string, isout bool) error {
			if !isout {
				defer job.Rinse.Jaws.Dirty(uiJobStatus{job})
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
						job.progress = time.Now()
						break
					}
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
		if err = job.runsc(ctx, stdouthandler, args...); err != nil {
			if !(errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
				for _, s := range output {
					slog.Error("tesseract", "msg", s)
				}
			}
		}
	}
	return
}

func (job *Job) jobEnding(ctx context.Context) (err error) {
	if err = job.transition(ctx, JobTesseract, JobEnding); err == nil {
		if err = os.Rename(path.Join(job.Datadir, "output.pdf"), path.Join(job.Datadir, job.ResultName())); err == nil {
			var diskuse int64
			err = filepath.WalkDir(job.Datadir, func(fpath string, d fs.DirEntry, err error) error {
				if err == nil {
					if d.Type().IsRegular() {
						switch filepath.Ext(d.Name()) {
						case ".png", ".pdf", ".json":
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
			job.Rinse.Jaws.Dirty(job, uiJobStatus{job})
		}
	}
	return
}

package rinser

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"
)

func (rns *Rinse) SelfTest() int {
	const srcName = "selftest.html"
	const srcLang = "auto"
	srcFile, err := assetsFS.Open(path.Join("assets", srcName))
	if err == nil {
		defer srcFile.Close()
		var job *Job
		job, err = NewJob(rns, srcName, srcLang, 1, 60*20, 1, 60, true, true, "selftest@localhost")
		if err == nil {
			dstName := filepath.Clean(path.Join(job.Datadir, srcName))
			var dstFile *os.File
			if dstFile, err = os.Create(dstName); err == nil {
				defer dstFile.Close()
				if _, err = io.Copy(dstFile, srcFile); err == nil {
					if err = dstFile.Sync(); err == nil {
						if err = job.Start(); err == nil {
							to := time.NewTimer(time.Minute * 10)
							defer func() {
								to.Stop()
								if logfile, err := os.Open(job.LogPath()); err == nil {
									defer logfile.Close()
									fmt.Fprintf(os.Stdout, "\n\nlog file %q:\n", job.LogPath())
									_, _ = io.Copy(os.Stdout, logfile)
									fmt.Fprintln(os.Stdout)
								}
								job.Close(nil)
							}()

							select {
							case <-to.C:
								err = errors.New("timeout")
							case <-job.StoppedCh:
								if err = job.Error; err == nil {
									if job.HasMeta() {
										if job.HasLog() {
											if job.Lang() == "eng+swe" {
												var f *os.File
												if f, err = os.Open(job.ResultPath()); err == nil {
													defer f.Close()
													var written int64
													if written, err = io.Copy(io.Discard, f); err == nil {
														if written > 0 {
															return 0
														} else {
															err = fmt.Errorf("%q is empty", job.ResultPath())
														}
													}
												}
											} else {
												err = fmt.Errorf("unexpected language %q", job.Lang())
											}
										} else {
											err = fmt.Errorf("%q not found", job.LogPath())
										}
									} else {
										err = fmt.Errorf("%q not found", job.MetaPath())
									}
								}
							}
						}
					}
				}
			}
		}
	}

	rns.Error("selftest", "err", err)
	return 1
}

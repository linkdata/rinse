package rinser

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"time"
)

func (rns *Rinse) SelfTest() int {
	const srcName = "selftest.html"
	const srcLang = "eng"
	srcFile, err := assetsFS.Open(path.Join("assets", srcName))
	if err == nil {
		defer srcFile.Close()
		var job *Job
		job, err = NewJob(rns, srcName, srcLang, 1, 60*20, 1, true, true, "selftest@localhost")
		if err == nil {
			dstName := filepath.Clean(path.Join(job.Datadir, srcName))
			var dstFile *os.File
			if dstFile, err = os.Create(dstName); err == nil {
				defer dstFile.Close()
				if _, err = io.Copy(dstFile, srcFile); err == nil {
					if err = dstFile.Sync(); err == nil {
						if err = job.Start(); err == nil {
							defer job.Close()
							to := time.NewTimer(time.Minute * 10)
							defer to.Stop()
							select {
							case <-to.C:
								err = errors.New("timeout")
							case <-job.StoppedCh:
								if err = job.Error; err == nil {
									if job.HasMeta() {
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

	slog.Error("selftest", "err", err)
	return 1
}

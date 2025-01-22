package rinser

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"text/template"
	"time"
)

type configJsonData struct {
	Args        string
	RootDir     string
	VarRinseDir string
	Uid         int
	Gid         int
}

func mustJson(obj any) string {
	b, err := json.Marshal(obj)
	if err == nil {
		return string(b)
	}
	panic(err)
}

var configJsonTmpl = template.Must(template.New("config.tmpl").ParseFS(assetsFS, "assets/config.tmpl"))

func runsc(ctx context.Context, runscBin, rootfsDir, workDir, logPath string, id string, outhandler func(string, bool) error, cmds ...string) (err error) {
	var logfile *os.File
	if logfile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600); err == nil /* #nosec G304 */ {
		defer logfile.Close()
		var f *os.File
		if f, err = os.Create(path.Join(workDir, "config.json")); err == nil /* #nosec G304 */ {
			defer f.Close()
			varRinseDir := path.Join(workDir, "data")
			isRoot := os.Getuid() == 0
			var uidgid int
			if isRoot {
				uidgid = 1000
			}
			cfg := &configJsonData{
				Args:        mustJson(cmds),
				RootDir:     mustJson(rootfsDir),
				VarRinseDir: mustJson(varRinseDir),
				Uid:         uidgid,
				Gid:         uidgid,
			}
			if err = os.MkdirAll(varRinseDir, 0777); err == nil /* #nosec G301 */ {
				if err = os.Chmod(varRinseDir, 0777); err == nil /* #nosec G302 */ {
					if err = configJsonTmpl.ExecuteTemplate(f, "config.tmpl", cfg); err == nil {
						if err = f.Close(); err == nil {
							runscargs := []string{"-ignore-cgroups", "-network", "none"}
							if !isRoot {
								runscargs = append(runscargs, "-rootless")
							}
							runscargs = append(runscargs, "run", "-bundle", workDir, id)
							fmt.Fprintf(logfile, "%v %s %v %v\n", time.Now().UTC().Format(time.DateTime), runscBin, runscargs, cmds)
							cmd := exec.Command(runscBin, runscargs...) // #nosec G204
							cmd.Dir = workDir
							defer func() {
								if cmd.Process != nil {
									if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
										if e := cmd.Process.Kill(); e != nil {
											panic(e)
										}
									}
									fmt.Fprintf(logfile, "%v %s exit code %v\n\n", time.Now().UTC().Format(time.DateTime), runscBin, cmd.ProcessState.ExitCode())
								}
							}()
							var stdout, stderr io.ReadCloser
							if stdout, err = cmd.StdoutPipe(); err == nil {
								if stderr, err = cmd.StderrPipe(); err == nil {
									if err = cmd.Start(); err == nil {
										outCh := make(chan string)
										errCh := make(chan string)
										go func() {
											defer close(outCh)
											lineScanner := bufio.NewScanner(stdout)
											for lineScanner.Scan() {
												select {
												case outCh <- lineScanner.Text():
												case <-ctx.Done():
													return
												}
											}
										}()
										go func() {
											defer close(errCh)
											lineScanner := bufio.NewScanner(stderr)
											for lineScanner.Scan() {
												select {
												case errCh <- lineScanner.Text():
												case <-ctx.Done():
													return
												}
											}
										}()

										for err == nil {
											select {
											case s, ok := <-outCh:
												if ok {
													fmt.Fprintf(logfile, "%v   %s\n", time.Now().UTC().Format(time.DateTime), s)
													if outhandler != nil {
														err = outhandler(s, true)
													}
												} else {
													if err = ctx.Err(); err == nil {
														if err = cmd.Wait(); err == nil {
															return
														}
													}
												}
											case s, ok := <-errCh:
												if ok {
													fmt.Fprintf(logfile, "%v   %s\n", time.Now().UTC().Format(time.DateTime), s)
													if outhandler != nil {
														err = outhandler(s, false)
													}
												}
											case <-ctx.Done():
												err = ctx.Err()
											}
										}
									}
								}
							}
							if err != nil {
								fmt.Fprintf(logfile, "%v %s error %q\n\n", time.Now().UTC().Format(time.DateTime), runscBin, err.Error())
							}
						}
					}
				}
			}
		}
	}

	return
}

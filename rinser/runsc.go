package rinser

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"text/template"
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

func runsc(ctx context.Context, rootfsDir, workDir string, id string, stdouthandler func(string) error, cmds ...string) (err error) {
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
						slog.Info("runsc", "args", runscargs, "cmd", cmds)
						cmd := exec.Command("runsc", runscargs...) // #nosec G204
						cmd.Dir = workDir
						var errlines []string
						defer func() {
							if cmd.Process != nil {
								if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
									if e := cmd.Process.Kill(); e != nil {
										slog.Error("runsc kill failed", "err", e)
									}
								}
							}
						}()
						var stdout, stderr io.ReadCloser
						if stdout, err = cmd.StdoutPipe(); err == nil {
							if stderr, err = cmd.StderrPipe(); err == nil {
								if err = cmd.Start(); err == nil {
									lineCh := make(chan string)
									go func() {
										defer close(lineCh)
										lineScanner := bufio.NewScanner(stdout)
										for lineScanner.Scan() {
											s := lineScanner.Text()
											select {
											case lineCh <- s:
											case <-ctx.Done():
												return
											}
										}
									}()
									go func() {
										lineScanner := bufio.NewScanner(stderr)
										for lineScanner.Scan() {
											errlines = append(errlines, lineScanner.Text())
										}
									}()

									for err == nil {
										select {
										case s, ok := <-lineCh:
											if !ok {
												if err = ctx.Err(); err == nil {
													if err = cmd.Wait(); err == nil {
														return
													}
												}
											} else if stdouthandler != nil {
												err = stdouthandler(s)
											}
										case <-ctx.Done():
											err = ctx.Err()
										}
									}
								}
							}
						}
						if err != nil {
							if !(errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
								slog.Error("runsc", "err", err)
								for _, s := range errlines {
									slog.Error("runsc", "stderr", s)
								}
							}
						}
					}
				}
			}
		}
	}

	return
}

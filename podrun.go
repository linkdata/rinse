package rinse

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"os/exec"
)

func podrun(ctx context.Context, podmanBin, runscBin, workDir string, stdouthandler func(string) error, cmds ...string) (err error) {
	var podmanargs []string
	if runscBin != "" {
		podmanargs = append(podmanargs, "--runtime="+runscBin)
	}
	podmanargs = append(podmanargs, "run", "--rm",
		"--log-driver", "none",
		"--security-opt", "no-new-privileges",
		"--security-opt", "label=type:container_engine_t",
		"--read-only",
		"--cap-drop", "all",
	)
	if cmds[0] != "wget" {
		podmanargs = append(podmanargs, "--network=none")
	}
	if runscBin == "" {
		podmanargs = append(podmanargs, "--userns=keep-id:uid=1000,gid=1000")
	} else {
		podmanargs = append(podmanargs, "--cap-add", "SYS_CHROOT")
	}
	if stdouthandler != nil {
		podmanargs = append(podmanargs, "--tty")
	}
	if workDir != "" {
		podmanargs = append(podmanargs, "-v", workDir+":/var/rinse")
	}
	podmanargs = append(podmanargs, PodmanImage)
	podmanargs = append(podmanargs, cmds...)

	slog.Debug("podman", "args", podmanargs)

	cmd := exec.Command(podmanBin, podmanargs...) // #nosec G204
	var stdout io.ReadCloser
	if stdout, err = cmd.StdoutPipe(); err == nil {
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
			for err == nil {
				select {
				case s, ok := <-lineCh:
					if !ok {
						if err = ctx.Err(); err == nil {
							return cmd.Wait()
						}
					}
					if stdouthandler != nil {
						err = stdouthandler(s)
					}
				case <-ctx.Done():
					err = ctx.Err()
				}
			}
			if cmd.Process != nil {
				if e := cmd.Process.Kill(); e != nil {
					slog.Error("podman kill failed", "err", e)
				}
			}
		}
	}
	if err != nil {
		if !(errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
			slog.Error("podman", "err", err)
		}
	}
	return
}

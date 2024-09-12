package rinse

//go:generate go run github.com/cparta/makeversion/cmd/mkver@latest -name rinse -out version.gen.go

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/staticserve"
	"github.com/linkdata/webserv"
)

//go:embed assets
var assetsFS embed.FS

const PodmanImage = "ghcr.io/linkdata/rinse"

type Rinse struct {
	Config        *webserv.Config
	Jaws          *jaws.Jaws
	PodmanBin     string
	RunscBin      string
	FaviconURI    string
	Languages     []string
	mu            deadlock.Mutex // protects following
	maxUploadSize int64
	autoCleanup   int
	jobs          []*Job
}

func New(cfg *webserv.Config, mux *http.ServeMux, jw *jaws.Jaws, maybePull bool) (rns *Rinse, err error) {
	var tmpl *template.Template
	var faviconuri string
	if tmpl, err = template.New("").ParseFS(assetsFS, "assets/ui/*.html"); err == nil {
		jw.AddTemplateLookuper(tmpl)
		var extraFiles []string
		addStaticFiles := func(filename string, ss *staticserve.StaticServe) (err error) {
			uri := path.Join("/static", ss.Name)
			if strings.HasSuffix(filename, "favicon.png") {
				faviconuri = uri
			}
			extraFiles = append(extraFiles, uri)
			mux.Handle(uri, ss)
			return
		}
		if err = os.MkdirAll(cfg.DataDir, 0775); err == nil {
			if err = staticserve.WalkDir(assetsFS, "assets/static", addStaticFiles); err == nil {
				if err = jw.GenerateHeadHTML(extraFiles...); err == nil {
					var podmanBin string
					if podmanBin, err = exec.LookPath("podman"); err == nil {
						slog.Info("podman", "bin", podmanBin)
						var runscbin string
						if s, e := exec.LookPath("runsc"); e == nil {
							if os.Getuid() == 0 && cfg.User == "" {
								runscbin = s
								slog.Info("gVisor", "bin", runscbin)
							} else {
								slog.Warn("gVisor needs root", "bin", s)
							}
						} else {
							slog.Info("gVisor not found", "err", e)
						}
						if err = maybePullImage(maybePull, podmanBin); err == nil {
							var langs []string
							if langs, err = getLanguages(podmanBin, []string{"eng", "swe"}); err == nil {
								rns = &Rinse{
									Config:        cfg,
									Jaws:          jw,
									PodmanBin:     podmanBin,
									RunscBin:      runscbin,
									FaviconURI:    faviconuri,
									maxUploadSize: 1024 * 1024 * 1024, // 1Gb
									autoCleanup:   60 * 24,            // 1 day
									Languages:     langs,
								}
								rns.addRoutes(mux)
								if e := rns.loadSettings(); e != nil {
									slog.Error("loadSettings", "err", e)
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

func (rns *Rinse) addRoutes(mux *http.ServeMux) {
	mux.Handle("GET /{$}", rns.Jaws.Handler("index.html", rns))
	mux.Handle("GET /setup/{$}", rns.Jaws.Handler("setup.html", rns))
	mux.Handle("GET /api/{$}", rns.Jaws.Handler("api.html", rns))
	mux.HandleFunc("POST /job", rns.handlePostJob)
	mux.HandleFunc("POST /submit", rns.handlePostSubmit)
	mux.HandleFunc("GET /job/{uuid}", rns.handleGetJob)
	mux.HandleFunc("PUT /job/{file}", rns.handlePutJob)
	mux.HandleFunc("DELETE /job/{uuid}", rns.handleDeleteJob)
}

func (rns *Rinse) MaxUploadSize() (n int64) {
	rns.mu.Lock()
	n = rns.maxUploadSize
	rns.mu.Unlock()
	return
}

func (rns *Rinse) AutoCleanup() (n int) {
	rns.mu.Lock()
	n = rns.autoCleanup
	rns.mu.Unlock()
	return
}

func (rns *Rinse) Close() {
	rns.mu.Lock()
	jobs := rns.jobs
	rns.jobs = nil
	rns.mu.Unlock()
	for _, job := range jobs {
		job.Close()
	}
}

func maybePullImage(maybePull bool, podmanBin string) (err error) {
	if maybePull {
		err = pullImage(podmanBin)
	}
	return
}

func pullImage(podmanBin string) (err error) {
	img := PodmanImage + ":latest"
	slog.Info("pullImage", "image", img)
	var out []byte
	cmd := exec.Command(podmanBin, "pull", "-q", img)
	if out, err = cmd.CombinedOutput(); err != nil {
		for _, line := range bytes.Split(bytes.TrimSpace(out), []byte{'\n'}) {
			slog.Error("pullImage", "msg", string(bytes.TrimSpace(line)))
		}
	} else {
		slog.Info("pullImage", "result", string(bytes.TrimSpace(out)))
	}
	return
}

func getLanguages(podmanBin string, prioLangs []string) (langs []string, err error) {
	var msgs []string
	stdouthandler := func(line string) error {
		msgs = append(msgs, line)
		if strings.IndexByte(line, ' ') == -1 {
			lang := strings.TrimSpace(line)
			if _, ok := LanguageCode[lang]; ok {
				langs = append(langs, lang)
			}
		}
		return nil
	}
	if err = podrun(context.Background(), podmanBin, "", "", stdouthandler, "tesseract", "--list-langs"); err == nil {
		slices.SortFunc(langs, func(a, b string) int { return strings.Compare(LanguageCode[a], LanguageCode[b]) })
		for i := len(prioLangs) - 1; i >= 0; i-- {
			if idx := slices.Index(langs, prioLangs[i]); idx != -1 {
				langs = slices.Delete(langs, idx, idx+1)
				langs = append([]string{prioLangs[i]}, langs...)
			}
		}
	} else {
		for _, s := range msgs {
			slog.Error("getLanguages", "msg", s)
		}
	}
	return
}

func (rns *Rinse) PkgName() string {
	return PkgName
}

func (rns *Rinse) PkgVersion() string {
	return PkgVersion
}

func (rns *Rinse) NewJob(name, lang string) (job *Job, err error) {
	return NewJob(rns, name, lang)
}

func (rns *Rinse) nextJobLocked() (nextJob *Job) {
	for _, job := range rns.jobs {
		switch job.State() {
		case JobNew:
			if nextJob == nil {
				nextJob = job
			}
		case JobFailed, JobFinished:
		default:
			return nil
		}
	}
	return
}

func (rns *Rinse) nextJob() (nextJob *Job) {
	rns.mu.Lock()
	defer rns.mu.Unlock()
	return rns.nextJobLocked()
}

func (rns *Rinse) MaybeStartJob() (err error) {
	if job := rns.nextJob(); job != nil {
		err = job.Start()
	}
	return
}

var ErrDuplicateUUID = errors.New("duplicate UUID")

func (rns *Rinse) AddJob(job *Job) (err error) {
	rns.mu.Lock()
	defer rns.mu.Unlock()
	for _, j := range rns.jobs {
		if job.UUID == j.UUID {
			return ErrDuplicateUUID
		}
	}
	rns.jobs = append(rns.jobs, job)
	if nextJob := rns.nextJobLocked(); nextJob != nil {
		nextJob.Start()
	}
	rns.Jaws.Dirty(rns)
	return
}

func (rns *Rinse) RemoveJob(job *Job) {
	rns.mu.Lock()
	rns.jobs = slices.DeleteFunc(rns.jobs, func(x *Job) bool { return x == job })
	rns.mu.Unlock()
	rns.Jaws.Dirty(rns)
}

// JawsContains implements jaws.Container.
func (rns *Rinse) JawsContains(e *jaws.Element) (contents []jaws.UI) {
	var sortedJobs []*Job
	rns.mu.Lock()
	sortedJobs = append(sortedJobs, rns.jobs...)
	rns.mu.Unlock()
	slices.SortFunc(sortedJobs, func(a, b *Job) int { return b.Created.Compare(a.Created) })
	for _, job := range sortedJobs {
		contents = append(contents, jaws.NewTemplate("job.html", job))
	}
	return
}

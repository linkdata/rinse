package rinser

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
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/staticserve"
	"github.com/linkdata/webserv"
)

//go:embed assets
var assetsFS embed.FS

var ErrDuplicateUUID = errors.New("duplicate UUID")

const PodmanImage = "ghcr.io/linkdata/rinse"

type Rinse struct {
	Config        *webserv.Config
	Jaws          *jaws.Jaws
	PodmanBin     string
	RunscBin      string
	FaviconURI    string
	Languages     []string
	mu            deadlock.Mutex // protects following
	closed        bool
	maxUploadSize int64
	autoCleanup   int
	maxRuntime    int
	maxConcurrent int
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
		if err = os.MkdirAll(cfg.DataDir, 0775); err == nil { // #nosec G301
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
							if langs, err = getLanguages(podmanBin); err == nil {
								rns = &Rinse{
									Config:        cfg,
									Jaws:          jw,
									PodmanBin:     podmanBin,
									RunscBin:      runscbin,
									FaviconURI:    faviconuri,
									maxUploadSize: 1024 * 1024 * 1024, // 1Gb
									autoCleanup:   60 * 24,            // 1 day
									maxRuntime:    60 * 60,            // 1 hour
									maxConcurrent: 2,
									jobs:          make([]*Job, 0),
									Languages:     langs,
								}
								rns.addRoutes(mux)
								if e := rns.loadSettings(); e != nil {
									slog.Error("loadSettings", "file", rns.settingsFile(), "err", e)
								}
								go rns.runBackgroundTasks()
							}
						}
					}
				}
			}
		}
	}

	return
}

func (rns *Rinse) runTasks() (todo []*Job) {
	rns.mu.Lock()
	defer rns.mu.Unlock()
	deadline := time.Minute * time.Duration(rns.autoCleanup)
	running := 0
	var nextJob *Job
	for _, job := range rns.jobs {
		switch job.State() {
		case JobNew:
			if nextJob == nil {
				nextJob = job
			}
		case JobFailed, JobFinished:
			if rns.autoCleanup > 0 && time.Since(job.stopped) > deadline {
				todo = append(todo, job)
			}
		default:
			running++
		}
	}
	if nextJob != nil && running < rns.maxConcurrent {
		if err := nextJob.Start(time.Duration(rns.maxRuntime) * time.Second); err != nil {
			slog.Error("startjob", "job", nextJob.Name, "err", err)
		}
	}
	return
}

func (rns *Rinse) IsClosed() (yes bool) {
	rns.mu.Lock()
	yes = rns.closed
	rns.mu.Unlock()
	return
}

func (rns *Rinse) runBackgroundTasks() {
	for !rns.IsClosed() {
		time.Sleep(time.Second)
		for _, job := range rns.runTasks() {
			rns.RemoveJob(job)
		}
	}
}

func (rns *Rinse) addRoutes(mux *http.ServeMux) {
	mux.Handle("GET /{$}", rns.Jaws.Handler("index.html", rns))
	mux.Handle("GET /setup/{$}", rns.Jaws.Handler("setup.html", rns))
	mux.Handle("GET /api/{$}", rns.Jaws.Handler("api.html", rns))
	mux.HandleFunc("POST /submit", func(w http.ResponseWriter, r *http.Request) { rns.handlePost(true, w, r) })

	basePath := ""
	mux.HandleFunc("GET "+basePath+"/jobs", rns.RESTGETJobs)
	mux.HandleFunc("GET "+basePath+"/jobs/{uuid}", rns.RESTGETJobsUUID)
	mux.HandleFunc("GET "+basePath+"/jobs/{uuid}/preview", rns.RESTGETJobsUUIDPreview)
	mux.HandleFunc("GET "+basePath+"/jobs/{uuid}/rinsed", rns.RESTGETJobsUUIDRinsed)
	mux.HandleFunc("POST "+basePath+"/jobs", rns.RESTPOSTJobs)
	mux.HandleFunc("DELETE "+basePath+"/jobs/{uuid}", rns.RESTDELETEJobsUUID)
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

func (rns *Rinse) MaxRuntime() (n int) {
	rns.mu.Lock()
	n = rns.maxRuntime
	rns.mu.Unlock()
	return
}

func (rns *Rinse) MaxConcurrent() (n int) {
	rns.mu.Lock()
	n = rns.maxConcurrent
	rns.mu.Unlock()
	return
}

func (rns *Rinse) Close() {
	rns.mu.Lock()
	jobs := rns.jobs
	if !rns.closed {
		rns.closed = true
		rns.jobs = nil
	}
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

func getLanguages(podmanBin string) (langs []string, err error) {
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
	} else {
		for _, s := range msgs {
			slog.Error("getLanguages", "msg", s)
		}
	}
	return
}

func (rns *Rinse) PkgName() string {
	return "rinse"
}

func (rns *Rinse) PkgVersion() string {
	return PkgVersion
}

func (rns *Rinse) NewJob(name, lang string) (job *Job, err error) {
	return NewJob(rns, name, lang)
}

func (rns *Rinse) nextJobLocked() (nextJob *Job) {
	running := 0
	for _, job := range rns.jobs {
		switch job.State() {
		case JobNew:
			if nextJob == nil {
				nextJob = job
			}
		case JobFailed, JobFinished:
		default:
			running++
			if running >= rns.maxConcurrent {
				return nil
			}
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
		err = job.Start(time.Duration(rns.MaxRuntime()) * time.Second)
	}
	return
}

func (rns *Rinse) AddJob(job *Job) (err error) {
	rns.mu.Lock()
	defer rns.mu.Unlock()
	err = http.ErrServerClosed
	if !rns.closed {
		err = ErrDuplicateUUID
		for _, j := range rns.jobs {
			if job.UUID == j.UUID {
				return
			}
		}
		err = nil
		rns.jobs = append(rns.jobs, job)
		if nextJob := rns.nextJobLocked(); nextJob != nil {
			_ = nextJob.Start(time.Duration(rns.maxRuntime) * time.Second)
		}
		rns.Jaws.Dirty(rns)
	}
	return
}

func (rns *Rinse) RemoveJob(job *Job) {
	rns.mu.Lock()
	rns.jobs = slices.DeleteFunc(rns.jobs, func(x *Job) bool { return x == job })
	rns.mu.Unlock()
	job.Close()
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

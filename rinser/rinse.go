package rinser

import (
	"context"
	"embed"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/staticserve"
	"github.com/linkdata/webserv"
)

//go:embed assets
var assetsFS embed.FS

//go:generate go run github.com/cparta/makeversion/cmd/mkver@v1.7.0 -name rinser -out version.gen.go -release

var ErrDuplicateUUID = errors.New("duplicate UUID")

const WorkerImage = "ghcr.io/linkdata/rinseworker"

type Rinse struct {
	Config        *webserv.Config
	Jaws          *jaws.Jaws
	RunscBin      string
	RootDir       string
	FaviconURI    string
	Languages     []string
	mu            deadlock.Mutex // protects following
	closed        bool
	maxSizeMB     int
	cleanupSec    int
	maxTimeSec    int
	maxConcurrent int
	cleanupGotten bool
	jobs          []*Job
}

var ErrWorkerRootDirNotFound = errors.New("/opt/rinseworker not found")

func locateRootDir() (fp string, err error) {
	var fi os.FileInfo
	fp = "/opt/rinseworker"
	if fi, err = os.Stat(fp); err == nil && fi.IsDir() {
		return fp, nil
	}
	if fp, err = filepath.Abs(path.Join("rootfs", fp)); err == nil {
		if fi, err = os.Stat(fp); err == nil && fi.IsDir() {
			return fp, nil
		}
	}
	slog.Error("locateRootDir", "err", err)
	return "", ErrWorkerRootDirNotFound
}

func New(cfg *webserv.Config, mux *http.ServeMux, jw *jaws.Jaws, devel bool) (rns *Rinse, err error) {
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
		if err = os.MkdirAll(cfg.DataDir, 0750); err == nil { // #nosec G301
			if err = staticserve.WalkDir(assetsFS, "assets/static", addStaticFiles); err == nil {
				if err = jw.GenerateHeadHTML(extraFiles...); err == nil {
					var runscbin string
					if runscbin, err = exec.LookPath("runsc"); err == nil {
						var rootDir string
						if rootDir, err = locateRootDir(); err == nil {
							var langs []string
							if langs, err = getLanguages(rootDir); err == nil {
								rns = &Rinse{
									Config:     cfg,
									Jaws:       jw,
									RunscBin:   runscbin,
									RootDir:    rootDir,
									FaviconURI: faviconuri,
									jobs:       make([]*Job, 0),
									Languages:  langs,
								}
								rns.addRoutes(mux, devel)
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
	running := 0
	var nextJob *Job
	for _, job := range rns.jobs {
		switch job.State() {
		case JobNew:
			if nextJob == nil {
				nextJob = job
			}
		case JobFailed, JobFinished:
			if job.CleanupSec >= 0 && time.Since(job.Stopped()) > time.Duration(job.CleanupSec)*time.Second {
				todo = append(todo, job)
			}
		default:
			running++
		}
	}
	if nextJob != nil && running < rns.maxConcurrent {
		if err := nextJob.Start(); err != nil {
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

func (rns *Rinse) addRoutes(mux *http.ServeMux, devel bool) {
	mux.Handle("GET /{$}", rns.Jaws.Handler("index.html", rns))
	mux.Handle("GET /setup/{$}", rns.Jaws.Handler("setup.html", rns))
	mux.Handle("GET /about/{$}", rns.Jaws.Handler("about.html", rns))
	mux.HandleFunc("POST /submit", func(w http.ResponseWriter, r *http.Request) { rns.handlePost(true, w, r) })
	if !devel {
		mux.Handle("GET /api/{$}", rns.Jaws.Handler("api.html", rns))
		mux.Handle("GET /api/index.html", rns.Jaws.Handler("api.html", rns))
	}

	basePath := ""
	mux.HandleFunc("GET "+basePath+"/jobs", rns.RESTGETJobs)
	mux.HandleFunc("GET "+basePath+"/jobs/{uuid}", rns.RESTGETJobsUUID)
	mux.HandleFunc("GET "+basePath+"/jobs/{uuid}/preview", rns.RESTGETJobsUUIDPreview)
	mux.HandleFunc("GET "+basePath+"/jobs/{uuid}/rinsed", rns.RESTGETJobsUUIDRinsed)
	mux.HandleFunc("GET "+basePath+"/jobs/{uuid}/meta", rns.RESTGETJobsUUIDMeta)
	mux.HandleFunc("POST "+basePath+"/jobs", rns.RESTPOSTJobs)
	mux.HandleFunc("DELETE "+basePath+"/jobs/{uuid}", rns.RESTDELETEJobsUUID)
}

func (rns *Rinse) CleanupSec() (n int) {
	rns.mu.Lock()
	n = rns.cleanupSec
	rns.mu.Unlock()
	return
}

func (rns *Rinse) MaxTimeSec() (n int) {
	rns.mu.Lock()
	n = rns.maxTimeSec
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

func getLanguages(rootDir string) (langs []string, err error) {
	var msgs []string
	stdouthandler := func(line string, isout bool) error {
		if isout {
			msgs = append(msgs, line)
			if strings.IndexByte(line, ' ') == -1 {
				lang := strings.TrimSpace(line)
				if _, ok := LanguageCode[lang]; ok {
					langs = append(langs, lang)
				}
			}
		}
		return nil
	}

	id := uuid.New()
	workDir := path.Join(os.TempDir(), "rinse-"+id.String())
	if err = os.Mkdir(workDir, 0777); err == nil /* #nosec G301 */ {
		defer os.RemoveAll(workDir)
		if err = runsc(context.Background(), rootDir, workDir, id.String(), stdouthandler, "tesseract", "--list-langs"); err == nil {
			slices.SortFunc(langs, func(a, b string) int { return strings.Compare(LanguageCode[a], LanguageCode[b]) })
		} else {
			for _, s := range msgs {
				slog.Error("getLanguages", "msg", s)
			}
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
		err = job.Start()
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
			_ = nextJob.Start()
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

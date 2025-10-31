package rinser

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
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
	"github.com/linkdata/jaws/jawsboot"
	"github.com/linkdata/jaws/staticserve"
	"github.com/linkdata/jawsauth"
	"github.com/linkdata/rinse/jwt"
	"github.com/linkdata/webserv"
)

//go:embed assets
var assetsFS embed.FS

//go:generate go run github.com/linkdata/gitsemver@v1.9.0 -gopackage -name rinser -out rinser/version.gen.go

var ErrDuplicateUUID = errors.New("duplicate UUID")

const WorkerImage = "ghcr.io/linkdata/rinseworker"

type Rinse struct {
	Config          *webserv.Config
	Jaws            *jaws.Jaws
	JawsAuth        *jawsauth.Server
	RunscBin        string
	RootDir         string
	FaviconURI      string
	Languages       []string
	mu              deadlock.Mutex // protects following
	OAuth2Settings  jawsauth.Config
	closed          bool
	maxSizeMB       int
	maxTimeSec      int
	cleanupSec      int
	timeoutSec      int
	maxConcurrent   int
	cleanupGotten   bool
	jobs            []*Job
	proxyUrl        string
	externalIP      template.HTML
	admins          []string // admins from settings
	endpointForJWKs string
	JWTPublicKeys   jwt.JSONWebKeySet
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

func locateRunscBin(devel bool) (fp string, err error) {
	if devel || deadlock.Debug {
		var fi os.FileInfo
		if fp, err = filepath.Abs("rootfs/usr/bin/runsc"); err == nil {
			if fi, err = os.Stat(fp); err == nil && !fi.IsDir() {
				return fp, nil
			}
		}
	}
	return exec.LookPath("runsc")
}

func New(cfg *webserv.Config, mux *http.ServeMux, jw *jaws.Jaws, devel bool) (rns *Rinse, err error) {
	var tmpl *template.Template
	if tmpl, err = template.New("").ParseFS(assetsFS, "assets/ui/*.html"); err == nil {
		jw.AddTemplateLookuper(tmpl)
		if err = jw.Setup(mux.Handle, "/static",
			jawsboot.Setup,
			staticserve.MustNewFS(assetsFS, "assets/static", "images/favicon.png"),
		); err == nil {
			if err = os.MkdirAll(cfg.DataDir, 0750); err == nil { // #nosec G301
				var runscbin string
				if runscbin, err = locateRunscBin(devel); err == nil {
					var rootDir string
					if rootDir, err = locateRootDir(); err == nil {
						var langs []string
						if langs, err = getLanguages(runscbin, rootDir); err == nil {
							rns = &Rinse{
								Config:     cfg,
								Jaws:       jw,
								RunscBin:   runscbin,
								RootDir:    rootDir,
								FaviconURI: jw.FaviconURL(),
								jobs:       make([]*Job, 0),
								Languages:  langs,
							}
							if e := rns.loadSettings(); e != nil {
								rns.Error("loadSettings", "file", rns.SettingsFile(), "err", e)
							}
							var overrideUrl string
							if deadlock.Debug {
								overrideUrl = cfg.ListenURL
							}

							if rns.endpointForJWKs != "" {
								rns.JWTPublicKeys, err = jwt.GetJSONKeyWebSet(rns.endpointForJWKs)
								if err != nil {
									rns.Error("failed getting jwt public keys", "err", err)
								} else {
									rns.Info("fetched keys from", "endpoint", rns.endpointForJWKs)
								}
							} else {
								rns.Warn("No endpoint for fetching JWKs")
							}

							if rns.JawsAuth, err = jawsauth.NewDebug(jw, &rns.OAuth2Settings, mux.Handle, overrideUrl); err == nil {
								rns.JawsAuth.LoginEvent = func(sess *jaws.Session, hr *http.Request) {
									var adminstr string
									email := sess.Get(rns.JawsAuth.SessionEmailKey).(string)
									if rns.JawsAuth.IsAdmin(email) {
										adminstr = "admin "
									}
									rns.Info(adminstr+"login", "email", email)
								}
								rns.JawsAuth.LogoutEvent = func(sess *jaws.Session, hr *http.Request) {
									var adminstr string
									email := sess.Get(rns.JawsAuth.SessionEmailKey).(string)
									if rns.JawsAuth.IsAdmin(email) {
										adminstr = "admin "
									}
									rns.Info(adminstr+"logout", "email", email)
								}
								rns.setAdmins(rns.admins)
								rns.addRoutes(mux, devel)
								go rns.runBackgroundTasks()
								go rns.UpdateExternalIP()
								return
							}
							rns.Error("oauth", "err", err, "file", rns.SettingsFile())
						}
					}
				}
			}
		}
	}
	return
}

func (rns *Rinse) ContainerNotice() (s string) {
	if kind, ok := os.LookupEnv("container"); ok {
		if kind == "" {
			s = "(May be running inside a container.)"
		} else {
			s = fmt.Sprintf("(Inside %q container.)", kind)
		}
	}
	return
}

func (rns *Rinse) Error(msg string, keyValuePairs ...any) {
	if l := rns.Config.Logger; l != nil {
		l.Error(msg, keyValuePairs...)
	}
}

func (rns *Rinse) Warn(msg string, keyValuePairs ...any) {
	if l := rns.Config.Logger; l != nil {
		l.Warn(msg, keyValuePairs...)
	}
}

func (rns *Rinse) Info(msg string, keyValuePairs ...any) {
	if l := rns.Config.Logger; l != nil {
		l.Info(msg, keyValuePairs...)
	}
}

func (rns *Rinse) getClient() *http.Client {
	rns.mu.Lock()
	proxyUrl := rns.proxyUrl
	rns.mu.Unlock()
	if proxyUrl != "" {
		if u, err := url.Parse(proxyUrl); err == nil {
			if u.Scheme != "" && u.Host != "" {
				return &http.Client{
					Transport: &http.Transport{
						Proxy: func(r *http.Request) (*url.URL, error) {
							return u, nil
						},
					},
				}
			}
		}
	}
	return http.DefaultClient
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
			rns.Error("startjob", "job", nextJob.Name, "err", err)
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

func (rns *Rinse) GetEmail(hr *http.Request) (s string) {
	if email, ok := rns.Jaws.GetSession(hr).Get(rns.JawsAuth.SessionEmailKey).(string); ok {
		s = strings.TrimSpace(email)
	}
	return
}

func (rns *Rinse) IsAdmin(email string) (yes bool) {
	return rns.JawsAuth.IsAdmin(email)
}

func (rns *Rinse) getAdmins() (v []string) {
	v = rns.JawsAuth.GetAdmins()
	return
}

func (rns *Rinse) setAdmins(v []string) {
	rns.JawsAuth.SetAdmins(v)
}

func (rns *Rinse) ProxyURL() string {
	rns.mu.Lock()
	defer rns.mu.Unlock()
	return rns.proxyUrl
}

func (rns *Rinse) addRoutes(mux *http.ServeMux, devel bool) {
	mux.Handle("GET /{$}", rns.JawsAuth.Handler("index.html", rns))
	mux.Handle("GET /setup/{$}", rns.JawsAuth.HandlerAdmin("setup.html", rns))
	mux.Handle("GET /about/{$}", rns.JawsAuth.Handler("about.html", rns))
	mux.Handle("POST /submit", rns.RedirectAuthFn(func(w http.ResponseWriter, r *http.Request) { rns.handlePost(true, w, r) }))

	if !devel {
		mux.Handle("GET /api/{$}", rns.JawsAuth.Handler("api.html", rns))
		mux.Handle("GET /api/index.html", rns.JawsAuth.Handler("api.html", rns))
	}

	basePath := ""
	mux.Handle("GET "+basePath+"/jobs", rns.AuthFn(rns.RESTGETJobs))
	mux.Handle("GET "+basePath+"/jobs/{uuid}", rns.AuthFn(rns.RESTGETJobsUUID))
	mux.Handle("GET "+basePath+"/jobs/{uuid}/preview", rns.AuthFn(rns.RESTGETJobsUUIDPreview))
	mux.Handle("GET "+basePath+"/jobs/{uuid}/rinsed", rns.AuthFn(rns.RESTGETJobsUUIDRinsed))
	mux.Handle("GET "+basePath+"/jobs/{uuid}/meta", rns.AuthFn(rns.RESTGETJobsUUIDMeta))
	mux.Handle("GET "+basePath+"/jobs/{uuid}/log", rns.AuthFn(rns.RESTGETJobsUUIDLog))
	mux.Handle("POST "+basePath+"/jobs", rns.AuthFn(rns.RESTPOSTJobs))
	mux.Handle("DELETE "+basePath+"/jobs/{uuid}", rns.AuthFn(rns.RESTDELETEJobsUUID))
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

func (rns *Rinse) TimeoutSec() (n int) {
	rns.mu.Lock()
	n = rns.timeoutSec
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
		job.Close(nil)
	}
}

func getLanguages(runscBin, rootDir string) (langs []string, err error) {
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
		if err = runsc(context.Background(), runscBin, rootDir, workDir, path.Join(workDir, "lang.log"), id.String(), stdouthandler, "tesseract", "--list-langs"); err == nil {
			slices.SortFunc(langs, func(a, b string) int { return strings.Compare(LanguageCode[a], LanguageCode[b]) })
			slog.Info("getLanguages", "count", len(langs), "langs", langs)
		} else {
			slog.Error("getLanguages", "err", err)
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
	job.Close(nil)
	rns.Jaws.Dirty(rns)
}

// JawsContains implements jaws.Container.
func (rns *Rinse) JawsContains(e *jaws.Element) (contents []jaws.UI) {
	sortedJobs := rns.JobList(rns.GetEmail(e.Initial()))
	slices.SortFunc(sortedJobs, func(a, b *Job) int { return b.Created.Compare(a.Created) })
	for _, job := range sortedJobs {
		contents = append(contents, jaws.NewTemplate("job.html", job))
	}
	return
}

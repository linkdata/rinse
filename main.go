package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/webserv"
	"golang.org/x/net/ipv4"

	"github.com/linkdata/rinse/rinser"
)

//go:generate go run github.com/swaggo/swag/cmd/swag@latest init

//go:embed docs
var docsFS embed.FS

//	@title			rinse REST API
//	@version		1.0
//	@description	Document cleaning service API

var (
	flagListen   = flag.String("listen", os.Getenv("RINSE_LISTEN"), "serve HTTP requests on given [address][:port]")
	flagCertDir  = flag.String("certdir", os.Getenv("RINSE_CERTDIR"), "where to find fullchain.pem and privkey.pem")
	flagUser     = flag.String("user", os.Getenv("RINSE_USER"), "switch to this user after startup (*nix only)")
	flagDataDir  = flag.String("datadir", os.Getenv("RINSE_DATADIR"), "where to store data files after startup")
	flagSelfTest = flag.Bool("selftest", os.Getenv("RINSE_SELFTEST") != "", "run self-test")
	flagVersion  = flag.Bool("v", false, "display version")
)

func run() int {
	flag.Parse()

	if *flagVersion {
		fmt.Println(rinser.PkgVersion)
		return 0
	}

	certDir := *flagCertDir
	if certDir == "" {
		if _, err := os.Stat("/etc/certs/fullchain.pem"); err == nil {
			if _, err := os.Stat("/etc/certs/privkey.pem"); err == nil {
				certDir = "/etc/certs"
			}
		}
	}

	dataDir := *flagDataDir
	if dataDir == "" {
		if fi, err := os.Stat("/etc/rinse"); err == nil && fi.IsDir() {
			dataDir = "/etc/rinse"
		} else if os.Getuid() == 0 {
			dataDir = "/var/rinse"
		}
	}

	cfg := &webserv.Config{
		Address:              *flagListen,
		CertDir:              certDir,
		User:                 *flagUser,
		DataDir:              dataDir,
		DefaultDataDirSuffix: "rinse",
		Logger:               slog.Default(),
	}

	if *flagSelfTest {
		cfg.Address = "127.0.0.1:"
		cfg.DataDir = "/tmp/rinse-selftest"
	}

	jw := jaws.New()
	defer jw.Close()
	jw.Debug = deadlock.Debug
	jw.Logger = slog.Default()
	http.DefaultServeMux.Handle("/jaws/", jw)
	go jw.Serve()

	l, err := cfg.Listen()
	if err == nil {
		defer l.Close()
		var rns *rinser.Rinse
		if rns, err = rinser.New(cfg, http.DefaultServeMux, jw, RinseDevel); err == nil {
			defer rns.Close()

			if *flagSelfTest {
				return rns.SelfTest()
			}

			http.DefaultServeMux.HandleFunc("GET /docs/{fpath...}", func(w http.ResponseWriter, r *http.Request) {
				fpath := strings.TrimSuffix(r.PathValue("fpath"), "/")
				http.ServeFileFS(w, r, docsFS, path.Join("docs", fpath))
			})
			if port := os.Getenv("RINSE_PORT"); port != "" {
				s := cfg.ListenURL
				if idx := strings.LastIndexByte(s, ':'); idx > 6 {
					s = s[:idx]
				}
				cfg.ListenURL = s + ":" + strings.TrimPrefix(port, ":")
			}

			maybeSwagger(cfg.ListenURL)

			if err = cfg.Serve(context.Background(), l, http.DefaultServeMux); err == nil {
				return 0
			}
		}
	}
	slog.Error(err.Error())
	return 1
}

func main() {
	var _ ipv4.ICMPType // ensure direct dep on golang.org/x/net
	os.Exit(run())
}

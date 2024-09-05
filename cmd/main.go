package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"

	"git.cparta.dev/jli/rinse"
	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/webserv"
)

var (
	flagListen  = flag.String("listen", "", "serve HTTP requests on given [address][:port]")
	flagCertDir = flag.String("certdir", "", "where to find fullchain.pem and privkey.pem")
	flagUser    = flag.String("user", "", "switch to this user after startup (*nix only)")
	flagDataDir = flag.String("datadir", "", "where to store data files after startup")
	flagPull    = flag.Bool("pull", false, "pull latest versions of images")
	flagVersion = flag.Bool("v", false, "display version")
)

func main() {
	flag.Parse()

	if *flagVersion {
		fmt.Println(rinse.PkgVersion)
		return
	}

	cfg := &webserv.Config{
		Address:              *flagListen,
		CertDir:              *flagCertDir,
		User:                 *flagUser,
		DataDir:              *flagDataDir,
		DefaultDataDirSuffix: "rinse",
		Logger:               slog.Default(),
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
		var rns *rinse.Rinse
		if rns, err = rinse.New(cfg, http.DefaultServeMux, jw); err == nil {
			defer rns.Close()
			if err = rns.MaybePull(*flagPull); err == nil {
				var job *rinse.Job
				if job, err = rns.NewJob("test.pdf", "eng"); err == nil {
					var data []byte
					if data, err = os.ReadFile("testdata/input.pdf"); err == nil {
						if err = os.WriteFile(path.Join(job.Workdir, "test.pdf"), data, 0666); err == nil {
							rns.AddJob(job)
						}
					}
				}
				if err = cfg.Serve(context.Background(), l, http.DefaultServeMux); err == nil {
					return
				}
			}
		}
	}
	slog.Error(err.Error())
}

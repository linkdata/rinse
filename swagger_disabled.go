//go:build !devel && !debug && !race

package main

import "net/http"

const RinseDevel = false

func maybeSwagger(next http.Handler, listenUrl string) http.Handler { return next }

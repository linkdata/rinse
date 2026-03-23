//go:build !devel && !debug && !race

package main

import "net/http"

const RinseDevel = false

func maybeSwagger(mux *http.ServeMux, listenUrl string) {}

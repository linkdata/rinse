//go:build !devel && !debug && !race

package main

const RinseDevel = false

func maybeSwagger(listenUrl string) {
}

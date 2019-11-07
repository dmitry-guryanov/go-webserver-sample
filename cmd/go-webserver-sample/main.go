package main

import (
	"log"

	"github.com/dmitry-guryanov/go-webserver-sample/internal/pkg/httpserver"
	"github.com/dmitry-guryanov/go-webserver-sample/internal/pkg/wsnotify"

	"github.com/oklog/run"
)

func main() {
	hub := wsnotify.NewHub()

	srv, err := httpserver.New(hub, 8080)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	g := run.Group{}

	g.Add(
		srv.Run,
		func(error) {
			srv.Close()
		})

	g.Add(
		func() error {
			hub.Run()
			return nil
		},
		func(error) {
			hub.Close()
		})

	g.Run()
}

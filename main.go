package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"timetables/internal/api"
	"timetables/internal/application"
	"timetables/internal/repository"
)

func main() {
	ctx := context.Background()

	repo, err := repository.NewRepo(ctx)
	if err != nil {
		slog.Error("cannot connect to db", "err", err.Error())
	}

	server := application.NewServer(repo)

	r := http.NewServeMux()

	// get an `http.Handler` that we can use
	sh := api.NewStrictHandler(server, nil)
	h := api.HandlerFromMux(sh, r)
	s := &http.Server{
		Handler: h,
		Addr:    "0.0.0.0:81",
	}

	// And we serve HTTP until the world ends.
	log.Fatal(s.ListenAndServe())

}

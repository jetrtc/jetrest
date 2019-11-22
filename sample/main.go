package main

import (
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/jetrtc/log"
	"github.com/jetrtc/rest"
)

func main() {
	log := log.NewSugar(log.NewLogger(log.GoLogger(log.Debug, os.Stderr, "", log.LstdFlags)))
	log.Fatal(http.ListenAndServe("localhost:8080", router(log)))
}

func router(log log.Sugar) http.Handler {
	r := mux.NewRouter()
	rest := rest.NewServer(log)
	r.Path("/user/{id:[a-z]+}").Handler(rest.HandlerFunc(UserHandler))
	return r
}

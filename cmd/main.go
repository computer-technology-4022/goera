package main

import (
	"net/http"

	"github.com/computer-technology-4022/quera-clone/internal/config"
	handler "github.com/computer-technology-4022/quera-clone/internal/handlers"
)

func main() {
	http.Handle(config.StaticRouter, http.
		StripPrefix(config.StaticRouter, http.FileServer(http.Dir(config.StaticRouterDir))))
	http.HandleFunc("/", handler.WelcomeHandler)
	http.ListenAndServe(config.ServerPort, nil)
}

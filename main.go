package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/charmbracelet/log"
)

func main() {
	// flagDev := flag.Bool("dev", false, "Run in development mode. Compiles templates on every request.")
	// flag.Parse()

	logger := log.New(os.Stderr)
	logger.SetLevel(log.DebugLevel) //TODO: remove
	handler := newHandler(logger.WithPrefix("handler"))
	handler.prepareRoutes()

	log.Fatal(runServer(handler))
}

func runServer(h *handler) error {
	l := h.errorLogger.WithPrefix("http-server")

	srv := http.Server{
		Addr:    ":8080",
		Handler: h,
	}

	l.Info("Starting server", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("ListenAndServe: %s", err)
	}
	return fmt.Errorf("unexpected server shutdown")
}

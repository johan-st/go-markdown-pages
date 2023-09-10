package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"
)

const (
	gitRemote           = "https://github.com/johan-st/obsidian-vault"
	gitPath             = "../obsidian-vault"
	gitMdPath           = "../obsidian-vault/go-md-articles"
	gitRefreshIntervall = 1 * time.Hour
)

func main() {
	// flagDev := flag.Bool("dev", false, "Run in development mode. Compiles templates on every request.")
	// flag.Parse()

	l := log.New(os.Stderr)
	l.SetPrefix("go-md-server")
	l.SetReportTimestamp(true)
	l.SetLevel(log.DebugLevel)
	l.SetReportCaller(true)

	handler := newHandler(l)
	handler.prepareRoutes()

	// gitRefresher will pull from git at a given intervall
	go gitPoll(l, gitRefreshIntervall)

	log.Fatal(runServer(handler))
}

func runServer(h *handler) error {
	l := h.logger.WithPrefix("http-server")

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

// GIT HELPERS
func gitPoll(l *log.Logger, intervall time.Duration) {
	l = l.WithPrefix("git-refresher")
	l.Info("Starting git refresher", "intervall", intervall)

	for {
		l.Debug("git pull",
			"remote", gitRemote,
			"path", gitPath)
		err := gitPull(gitPath)
		if err != nil {
			l.Error("git pull", "err", err)
		}
		time.Sleep(intervall)
	}
}

func gitPull(gitPath string) error {
	cmd := exec.Command("git", "pull")
	cmd.Dir = gitPath
	return cmd.Run()

}

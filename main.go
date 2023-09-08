package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
)

const (
	gitRemote           = "https://github.com/johan-st/obsidian-vault"
	gitPath             = "../obsidian-vault"
	gitMdPath           = "../obsidian-vault/go-md-articles"
	gitRefreshIntervall = 1 * time.Minute
)

func main() {
	// flagDev := flag.Bool("dev", false, "Run in development mode. Compiles templates on every request.")
	// flag.Parse()

	logger := log.New(os.Stderr)
	logger.SetPrefix("go-md-server")
	logger.SetReportTimestamp(true)
	logger.SetLevel(log.DebugLevel)
	logger.SetReportCaller(true)

	handler := newHandler(logger)
	handler.prepareRoutes()

	log.Fatal(runServer(handler))
}

func runServer(h *handler) error {
	l := h.errorLogger.WithPrefix("http-server")

	srv := http.Server{
		Addr:    ":8080",
		Handler: h,
	}

	// gitRefresher will pull from git at a given intervall
	go gitRefresher(l, gitRefreshIntervall)

	l.Info("Starting server", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("ListenAndServe: %s", err)
	}
	return fmt.Errorf("unexpected server shutdown")
}

// GIT HELPERS
func gitRefresher(l *log.Logger, intervall time.Duration) {
	l = l.WithPrefix("git-refresher")
	l.Info("Starting git refresher", "intervall", intervall)

	absGitPath, err := filepath.Abs(gitPath)
	if err != nil {
		l.Error("filepath.Abs", "err", err)
		return
	}

	for {
		l.Debug("git pull",
			"remote", gitRemote,
			"path", absGitPath)
		err := gitPull(absGitPath)
		if err != nil {
			l.Error("git pull", "err", err)
		}
		time.Sleep(intervall)
	}
}

func gitPull(absGitPath string) error {
	cmd := exec.Command("git", "pull")
	cmd.Dir = absGitPath
	return cmd.Run()

}

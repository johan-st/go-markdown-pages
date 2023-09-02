package main

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/matryer/way"
	"gitlab.com/golang-commonmark/markdown"
)

type handler struct {
	errorLogger *log.Logger // *required
	// accessLogger *log.Logger // optional

	router way.Router
}

type page struct {
	// metadata
	title string
	path  string
	draft bool

	// content
	body string
}

func newHandler(l *log.Logger) *handler {
	h := &handler{
		errorLogger: l,
		router:      *way.NewRouter(),
	}

	return h
}

func (h *handler) prepareRoutes() {

	h.router.HandleFunc("GET", "", h.handleRoot())
}

// HANDLERS

func (h *handler) handleRoot() http.HandlerFunc {
	var fileName = "pages/index.md"

	// setup
	l := h.errorLogger.With("handler", "handleRoot")
	defer func(t time.Time) {
		l.Debug("setup done", "time", time.Since(t))
	}(time.Now())

	mdRenderer := markdown.New(markdown.XHTMLOutput(true))

	// handler
	return func(w http.ResponseWriter, r *http.Request) {
		// timer
		defer func(t time.Time) {
			l.Debug("response", "time", time.Since(t))
		}(time.Now())

		file, err := os.ReadFile(fileName)
		if err != nil {
			l.Error("read file", "error", err)
			respondStatus(w, r, http.StatusInternalServerError)
			return
		}
		err = mdRenderer.Render(w, file)
		if err != nil {
			l.Error("render markdown", "error", err)
			respondStatus(w, r, http.StatusInternalServerError)
			return
		}
	}
}

// RESPONDERS

func respondStatus(w http.ResponseWriter, r *http.Request, status int) {
	w.WriteHeader(status)
}

// PAGE

// TODO: implement
func preparePage(src io.Reader) (page, error) {
	p := page{title: "test", path: "test", draft: false, body: "# test\n\ntest test test"}

	return p, nil
}

// OTHER

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

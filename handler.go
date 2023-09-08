package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/log"
	"github.com/matryer/way"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

type handler struct {
	errorLogger *log.Logger

	router way.Router
}

type page struct {
	Meta metadata
	Html string
	// Debug info
	filename string
}
type metadata struct {
	Title string
	Path  string
	Draft bool
	Tags  []string
	Date  time.Time
}

var (
	// folder containing templates
	tmplFolder = "templates"

	// mdGlobs for markdown files
	mdGlobs = []string{
		"pages/*.md",
		"pages/**/*.md",
		gitMdPath + "/*.md",
	}
)

func newHandler(l *log.Logger) *handler {
	h := &handler{
		errorLogger: l,
		router:      *way.NewRouter(),
	}

	return h
}

func (h *handler) prepareRoutes() {

	h.router.HandleFunc("GET", "/partials/:template", h.handlePartialsDev())
	h.router.HandleFunc("GET", "/public/", h.handlePublic())
	h.router.HandleFunc("GET", "...", h.handleDevMode())

}

// HANDLERS

func (h *handler) handlePartialsDev() http.HandlerFunc {
	type colorsData string
	var (
		tmplData any

		// "colors"
		colors    = []string{"#EE6055", "#60D394", "#AAF683", "#FFD97D", "#FF9B85"}
		nextColor = 0
	)

	// setup
	l := h.errorLogger.With("handler", "handlePartials")
	defer func(t time.Time) {
		l.Debug("handler ready", "time", time.Since(t))
	}(time.Now())

	// handler
	return func(w http.ResponseWriter, r *http.Request) {
		// timer
		defer func(t time.Time) {
			l.Debug("responding",
				"time", time.Since(t),
				"request_path", r.URL.Path,
				"template_requested", way.Param(r.Context(), "template"),
			)
		}(time.Now())

		tmpl, err := template.ParseGlob(tmplFolder + "/*.html")
		if err != nil {
			l.Error("parse template", "error", err)
			respondStatus(w, r, http.StatusInternalServerError)
			return
		}

		requestedTemplate := way.Param(r.Context(), "template")

		switch requestedTemplate {
		case "Color-Swap-Demo":
			color := colors[nextColor]
			nextColor = (nextColor + 1) % len(colors)
			tmplData = colorsData(color)
		default:
			tmplData = nil
		}

		err = tmpl.ExecuteTemplate(w, requestedTemplate, tmplData)
		if err != nil {
			l.Error("execute template", "template", requestedTemplate, "error", err)
			respondStatus(w, r, http.StatusInternalServerError)
		}
	}
}
func (h *handler) handlePublic() http.HandlerFunc {
	var (
		publicFolder = "public"
		pathPrefix   = "/public/"
	)
	// setup
	l := h.errorLogger.With("handler", "handlePublic")
	defer func(t time.Time) {
		l.Debug("handler ready", "time", time.Since(t))
	}(time.Now())

	// handler
	return func(w http.ResponseWriter, r *http.Request) {
		// timer
		defer func(t time.Time) {
			l.Debug("responding",
				"time", time.Since(t),
				"request_path", r.URL.Path,
				"status", w.Header().Get("status"),
				"content_length", w.Header().Get("content-length"),
			)
		}(time.Now())

		r.URL.Path = strings.TrimPrefix(r.URL.Path, pathPrefix)

		http.FileServer(http.Dir(publicFolder)).ServeHTTP(w, r)

	}
}

// TODO: prepare handlers on startup and not on every request
// except for dev mode
func (h *handler) handleDevMode() http.HandlerFunc {

	type templateData struct {
		BaseTitle string
		Nav       map[string]string
		Meta      map[string]string

		CSS []string
		JS  []string

		Page page
	}

	var (
		tmplData = templateData{
			Meta:      map[string]string{"description": "TODO: add description", "keywords": "TODO: add keywords"},
			BaseTitle: "go-md-server",

			Nav: map[string]string{},
		}
	)

	// setup
	l := h.errorLogger.With("handler", "handlePage")
	defer func(t time.Time) {
		l.Debug("handler ready", "time", time.Since(t))
	}(time.Now())

	// mdRenderer := markdown.New(markdown.XHTMLOutput(true), markdown.HTML(true))
	mdRenderer := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
		),
	)
	// handler
	return func(w http.ResponseWriter, r *http.Request) {
		// delay := time.Duration(50 * time.Millisecond)

		// timer
		defer func(t time.Time) {
			l.Info("response", "time", time.Since(t)) // "delay", delay,

		}(time.Now())

		// delay
		// time.Sleep(delay)

		// Find all markdown files
		var mdPaths []string
		for _, g := range mdGlobs {
			paths, err := filepath.Glob(g)
			if err != nil {
				h.errorLogger.Fatal("glob markdown files", "error", err)
			}
			mdPaths = append(mdPaths, paths...)
		}
		h.errorLogger.Debug("found markdown files", "files", mdPaths)

		// prepare template
		tmpl, err := template.ParseGlob(tmplFolder + "/*.html")
		if err != nil {
			l.Error("parse template", "error", err)
			respondStatus(w, r, http.StatusInternalServerError)
			return
		}

		// find page correscponding to path
		// path := r.URL.Path
		// for

		// prepare pages
		var pages []page
		tmplData.Nav = make(map[string]string, len(mdPaths))
		for _, path := range mdPaths {
			p, err := preparePage(mdRenderer, path)
			if err != nil {
				l.Error("prepare page. page ignored", "file", path, "error", err)
				continue
			}
			pages = append(pages, p)
			tmplData.Nav[p.Meta.Path] = p.Meta.Title

		}

		for _, p := range pages {
			pagePath, err := url.Parse(p.Meta.Path)
			if err != nil {
				l.Fatal("parse page path", "page", p, "error", err)
				continue
			}
			requestPath, err := url.Parse(r.URL.Path)
			if err != nil {
				l.Fatal("parse request path", "path", r.URL.Path, "error", err)
			}

			if pagePath.Path == requestPath.Path {

				// TODO: remove debug
				l.Debug("serving page",
					"path", p.Meta.Path,
					"title", p.Meta.Title,
					"filename", p.filename,
					"html_length", len(p.Html))

				tmplData.Page = p
				err = tmpl.ExecuteTemplate(w, "layout", tmplData)
				if err != nil {
					l.Error("execute template", "error", err)
					respondStatus(w, r, http.StatusInternalServerError)
				}
				return
			}
		}
		requestPath, err := url.Parse(r.URL.Path)
		if err != nil {
			l.Fatal("parse request path", "path", r.URL.Path, "error", err)
		}
		pagePaths := make(map[string]string, len(pages))
		for _, p := range pages {
			pagePaths[p.Meta.Path] = p.Meta.Title
		}

		l.Debug("request path does not match any page",
			"request_path", requestPath,
			"response_code", http.StatusNotFound,
			"avaiable_paths", pagePaths,
		)
		respondStatus(w, r, http.StatusNotFound)
	}
}

// RESPONDERS

func respondStatus(w http.ResponseWriter, r *http.Request, status int) {
	w.WriteHeader(status)
}

// PAGE

func preparePage(md goldmark.Markdown, path string) (page, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return page{}, fmt.Errorf("read file: %s", err)
	}

	var html bytes.Buffer
	ctx := parser.NewContext()
	err = md.Convert(file, &html, parser.WithContext(ctx))
	if err != nil {
		return page{}, fmt.Errorf("convert markdown: %s", err)
	}

	pageMeta, err := parseMetadata(meta.Get(ctx))
	if err != nil {
		return page{}, fmt.Errorf("parse metadata: %s", err)
	}
	p := page{
		Meta:     pageMeta,
		Html:     html.String(),
		filename: path,
	}
	return p, nil
}

func parseMetadata(meta map[string]any) (metadata, error) {
	m := metadata{}

	// REQUIRED fields
	if val, ok := meta["title"]; ok {
		m.Title = val.(string)
	} else {
		return m, fmt.Errorf("no title found")
	}

	if val, ok := meta["path"]; ok {
		m.Path = val.(string)
	} else {
		return m, fmt.Errorf("no path found")
	}

	if val, ok := meta["draft"]; ok {
		m.Draft = val.(bool)
	} else {
		return m, fmt.Errorf("no draft found")
	}

	// OPTIONAL fields
	m.Tags = []string{}
	m.Date = time.Time{}

	return m, nil
}

// OTHER

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

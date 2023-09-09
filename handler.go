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
	"github.com/yuin/goldmark/renderer/html"
)

type handler struct {
	errorLogger *log.Logger

	router way.Router
}

type article struct {
	Meta metadata
	Html string
	// Debug info
	filename string
}
type metadata struct {
	Title string
	Slug  string
	Draft bool
	Tags  []string
	Date  time.Time
}

var (
	// folder containing templates
	tmplFolder = "templates"
)

func newHandler(l *log.Logger) *handler {
	h := &handler{
		errorLogger: l,
		router:      *way.NewRouter(),
	}

	return h
}

func (h *handler) prepareRoutes() {
	h.router.HandleFunc("GET", "/git", h.handleGitWebhook())
	h.router.HandleFunc("GET", "/partials/:template", h.handlePartialsDev())
	h.router.HandleFunc("GET", "/public/", h.handlePublic())
	h.router.HandleFunc("GET", "/blog/:slug", h.handleBlogDev(gitMdPath, "blog"))

}

// HANDLERS

// handleGitWebhook handles executes a git pull on the gitPath when called
func (h *handler) handleGitWebhook() http.HandlerFunc {
	// setup
	l := h.errorLogger.With("handler", "handleGitPull")
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
			)
		}(time.Now())

		l.Debug("git pull",
			"remote", gitRemote,
			"path", gitPath)

		err := gitPull(gitPath)
		if err != nil {
			l.Error("git pull", "err", err)
			respondStatus(w, r, http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handlePartialsDev serves template partials. (dev: It reloads the templates on every request.)
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

// handleBlogDev serves the blog. It populates with files matching '*.md' in the given directories. TODO: specify frontmatter (dev: It reloads the templates on every request.)
func (h *handler) handleBlogDev(paths ...string) http.HandlerFunc {
	type templateData struct {
		BaseTitle string
		Nav       map[string]string
		Meta      map[string]string

		Article article
	}

	var (
		tmplData = templateData{
			Meta:      map[string]string{"description": "TODO: add description", "keywords": "TODO: add keywords"},
			BaseTitle: "jst.dev",

			Nav: map[string]string{},
		}
	)

	// setup
	l := h.errorLogger.With("handler", "handlePage")
	defer func(t time.Time) {
		l.Debug("handler ready", "time", time.Since(t))
	}(time.Now())

	// TODO: remove
	l.Debug("handleBlogDev setup", "paths", paths)
	if len(paths) == 0 {
		l.Fatal("handleBlogDev setup. no paths given")
	}
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
	// handler
	return func(w http.ResponseWriter, r *http.Request) {
		defer func(t time.Time) {
			l.Info("response", "time", time.Since(t))
		}(time.Now())

		slug := way.Param(r.Context(), "slug")
		l.Debug("handleBlogDev", "slug", slug)

		// Find all markdown files
		var mdPaths []string
		for _, p := range paths {
			paths, err := filepath.Glob(p + "/*.md")
			if err != nil {
				h.errorLogger.Fatal("glob markdown files", "error", err)
			}
			mdPaths = append(mdPaths, paths...)
		}
		h.errorLogger.Debug("found markdown files", "files", mdPaths)

		// prepare templates
		tmpl, err := template.ParseGlob(tmplFolder + "/*.html")
		if err != nil {
			l.Error("parse template", "error", err)
			respondStatus(w, r, http.StatusInternalServerError)
			return
		}

		// prepare pages
		var pages []article
		tmplData.Nav = make(map[string]string, len(mdPaths))
		for _, path := range mdPaths {
			p, err := preparePage(md, path)
			if err != nil {
				l.Error("prepare page. page ignored", "file", path, "error", err)
				continue
			}
			pages = append(pages, p)
			tmplData.Nav[p.Meta.Slug] = p.Meta.Title

		}

		for _, p := range pages {
			if slug == p.Meta.Slug {
				// TODO: remove debug
				l.Debug("serving page",
					"path", p.Meta.Slug,
					"title", p.Meta.Title,
					"filename", p.filename,
					"html_length", len(p.Html))

				tmplData.Article = p
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
			pagePaths[p.Meta.Slug] = p.Meta.Title
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

func preparePage(md goldmark.Markdown, path string) (article, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return article{}, fmt.Errorf("read file: %s", err)
	}

	var html bytes.Buffer
	ctx := parser.NewContext()
	err = md.Convert(file, &html, parser.WithContext(ctx))
	if err != nil {
		return article{}, fmt.Errorf("convert markdown: %s", err)
	}

	pageMeta, err := parseMetadata(meta.Get(ctx))
	if err != nil {
		return article{}, fmt.Errorf("parse metadata: %s", err)
	}
	p := article{
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

	if val, ok := meta["slug"]; ok {
		m.Slug = val.(string)
	} else {
		return m, fmt.Errorf("no slug found")
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

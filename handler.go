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
	"gitlab.com/golang-commonmark/markdown"
)

type handler struct {
	errorLogger *log.Logger

	router way.Router
}

type page struct {
	// metadata
	Meta map[string]string
	// content
	Html string

	// Debug info
	filename string
}

type templateData struct {
	BaseTitle string
	Nav       map[string]string
	Meta      map[string]string

	CSS []string
	JS  []string

	Page page
}

// list of required metadata keys. These are validated in parseMetadata
var requiredMeta = []string{"title", "path", "draft"}

func newHandler(l *log.Logger) *handler {
	h := &handler{
		errorLogger: l,
		router:      *way.NewRouter(),
	}

	return h
}

func (h *handler) prepareRoutes() {

	h.router.HandleFunc("GET", "/public/", h.handlePublic())
	h.router.HandleFunc("GET", "...", h.handleDevMode())

}

// HANDLERS

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
		// defer func(t time.Time) {
		// 	l.Debug("responding",
		// 		"time", time.Since(t),
		// 		"request_path", r.URL.Path,
		// 		"status", w.Header().Get("status"),
		// 		"content_length", w.Header().Get("content-length"),
		// 	)
		// }(time.Now())

		r.URL.Path = strings.TrimPrefix(r.URL.Path, pathPrefix)

		http.FileServer(http.Dir(publicFolder)).ServeHTTP(w, r)

	}
}

// TODO: prepare handlers on startup and not on every request
// except for dev mode
func (h *handler) handleDevMode() http.HandlerFunc {
	var (
		tmplFolder = "templates"

		globs = []string{
			"pages/*.md",
			"pages/**/*.md",
			"pages/**/**/*.md",
		}

		tmplData = templateData{
			Meta:      map[string]string{"description": "TODO: add description", "keywords": "TODO: add keywords"},
			BaseTitle: "go-md-server",

			Nav: map[string]string{"NOT POPULATED": "NOT POPULATED"},

			CSS: []string{"/public/tailwind.css"},
			JS: []string{"/public/main.js",
				// "<script src="https://unpkg.com/htmx.org@1.9.5" integrity="sha384-xcuj3WpfgjlKF+FXhSQFQ0ZNr39ln+hwjN3npfM9VBnUskLolQAcN80McRIVOPuO" crossorigin="anonymous"></script>"},
				"https://unpkg.com/htmx.org@1.9.5",
			},
		}
	)

	// setup
	l := h.errorLogger.With("handler", "handlePage")
	defer func(t time.Time) {
		l.Debug("handler ready", "time", time.Since(t))
	}(time.Now())

	mdRenderer := markdown.New(markdown.XHTMLOutput(true))

	// handler
	return func(w http.ResponseWriter, r *http.Request) {
		// timer
		defer func(t time.Time) {
			l.Info("response", "time", time.Since(t))
		}(time.Now())

		// Find all markdown files
		var mdPaths []string
		for _, g := range globs {
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
				l.Error("prepare page. Page will be ignored", "file", path, "error", err)
				continue
			}
			pages = append(pages, p)
			tmplData.Nav[p.Meta["path"]] = p.Meta["title"]

		}

		for _, p := range pages {
			pagePath, err := url.Parse(p.Meta["path"])
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
					"path", p.Meta["path"],
					"title", p.Meta["title"],
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
			pagePaths[p.Meta["path"]] = p.Meta["title"]
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

// TODO: implement and test
func preparePage(md *markdown.Markdown, path string) (page, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return page{}, fmt.Errorf("read file: %s", err)
	}

	mdMeta, mdBody, err := splitMarkdown(file)
	if err != nil {
		return page{}, fmt.Errorf("split markdown: %s", err)
	}

	pageMeta, err := parseMetadata(mdMeta)
	if err != nil {
		return page{}, fmt.Errorf("parse metadata: %s", err)
	}

	pageMeta["path"] = strings.ToLower(pageMeta["path"])

	html := md.RenderToString(mdBody)
	p := page{
		Meta:     pageMeta,
		Html:     html,
		filename: path,
	}

	return p, nil
}

func splitMarkdown(src []byte) ([]byte, []byte, error) {
	var sep = []byte("---")

	split := bytes.SplitN(src, sep, 3)
	if len(split) != 3 {
		return nil, nil, fmt.Errorf("invalid markdown file. Missing metadata")
	}
	return split[1], split[2], nil
}

func parseMetadata(src []byte) (map[string]string, error) {

	meta := make(map[string]string)
	src = bytes.ReplaceAll(src, []byte("\r"), []byte(""))

	// TODO: Clean up
	lines := bytes.Split(src, []byte("\n"))

	for _, l := range lines {
		keyVal := bytes.SplitN(l, []byte(":"), 2)
		if len(keyVal) != 2 {
			continue
		}

		meta[string(bytes.TrimSpace(keyVal[0]))] = string(bytes.TrimSpace(keyVal[1]))
	}

	for _, k := range requiredMeta {
		val, ok := meta[k]
		if !ok {
			return nil, fmt.Errorf("missing required metadata: %s", k)
		}
		if val == "" {
			return nil, fmt.Errorf("missing required metadata: %s", k)
		}
	}
	return meta, nil
}

// OTHER

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

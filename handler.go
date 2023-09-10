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
	logger *log.Logger

	router way.Router
}

type article struct {
	Meta     metadata
	Html     string
	filename string
}

type metadata struct {
	Title   string
	Slug    string
	Draft   bool
	Tags    []string
	Excerpt string
	Date    time.Time
}

type templateData struct {
	DocTitle string
	Nav      map[string]string
	Meta     map[string]string

	Content any
}

var (
	// folder containing templates
	tmplLayoutFolder = "templates/layout"
	tmplPageFolder   = "templates/pages"

	// base info for all pages
	mainNav = map[string]string{
		"/blog/docs": "docs",
		"/blog":      "blog",
	}
	baseTitle = "jst.dev"
)

func newHandler(l *log.Logger) *handler {
	h := &handler{
		logger: l,
		router: *way.NewRouter(),
	}

	return h
}

func (h *handler) prepareRoutes() {
	h.router.HandleFunc("GET", "/git", h.handleGitWebhook())
	h.router.HandleFunc("GET", "/partials/:template", h.handlePartialsDev())
	h.router.HandleFunc("GET", "/public/", h.handlePublic())
	h.router.HandleFunc("GET", "/blog", h.handlePageDev("blog.html", h.dataBlogIndex(gitMdPath+"/*.md", "blog/*.md")))
	h.router.HandleFunc("GET", "/blog/:slug", h.handleBlogDev(gitMdPath+"/*.md", "blog/*.md"))

	h.router.HandleFunc("GET", "...", h.handlePageDev("blog.html", h.dataBlogIndex(gitMdPath+"/*.md", "blog/*.md"))) //TODO: remove

}

// HANDLERS
func (h *handler) handlePageDev(contentTemplatePath string, getContentData func() any) http.HandlerFunc {
	// setup
	l := h.logger.With("handler", "handleContentDev")
	defer func(t time.Time) {
		l.Debug("handler ready", "time", time.Since(t))
	}(time.Now())

	// handler
	return func(w http.ResponseWriter, r *http.Request) {
		baseTmpls, err := template.ParseGlob(tmplLayoutFolder + "/*.html")
		if err != nil {
			l.Error("parse base template", "error", err)
			respondStatus(w, r, http.StatusInternalServerError)
			return
		}
		contentTmpls, err := baseTmpls.Clone()
		if err != nil {
			l.Error("clone base templates", "error", err)
			respondStatus(w, r, http.StatusInternalServerError)
			return
		}

		contentTmpls, err = contentTmpls.ParseFiles(tmplPageFolder + "/" + contentTemplatePath)
		if err != nil {
			l.Error("parse content template", "error", err)
			respondStatus(w, r, http.StatusInternalServerError)
			return
		}

		// prepare data
		tmplData := templateData{
			Meta: map[string]string{
				"description": "TODO: add description",
				"keywords":    "TODO: add keywords",
			},
			DocTitle: baseTitle + " | blog",
			Nav:      mainNav,

			Content: getContentData(),
		}

		// execute template
		err = contentTmpls.ExecuteTemplate(w, "layout", tmplData)
		if err != nil {
			l.Error("execute template", "error", err)
			respondStatus(w, r, http.StatusInternalServerError)
		}

	}
}

// handleGitWebhook handles executes a git pull on the gitPath when called
func (h *handler) handleGitWebhook() http.HandlerFunc {
	// setup
	l := h.logger.With("handler", "handleGitPull")
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
	l := h.logger.With("handler", "handlePartials")
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

		tmpl, err := template.ParseGlob(tmplLayoutFolder + "/*.html")
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
	l := h.logger.With("handler", "handlePublic")
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
func (h *handler) handleBlogDev(mdGlobs ...string) http.HandlerFunc {

	var (
		tmplData = templateData{
			Meta: map[string]string{
				"description": "TODO: add description",
				"keywords":    "TODO: add keywords",
			},
			DocTitle: baseTitle,

			Nav: mainNav,
		}
	)

	// setup
	l := h.logger.With("handler", "handlePage")
	defer func(t time.Time) {
		l.Debug("handler ready", "time", time.Since(t))
	}(time.Now())

	// TODO: remove
	l.Debug("handleBlogDev setup", "globs", mdGlobs)
	if len(mdGlobs) == 0 {
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
		for _, g := range mdGlobs {
			paths, err := filepath.Glob(g)
			if err != nil {
				h.logger.Fatal("glob markdown files", "error", err)
			}
			mdPaths = append(mdPaths, paths...)
		}
		h.logger.Debug("found markdown files", "files", mdPaths)

		// prepare templates
		baseTmpl, err := template.ParseGlob(tmplLayoutFolder + "/*.html")
		if err != nil {
			l.Error("parse template", "error", err)
			respondStatus(w, r, http.StatusInternalServerError)
			return
		}

		// prepare articles
		var articles []article
		for _, path := range mdPaths {
			p, err := preparePage(md, path)
			if err != nil {
				l.Error("prepare page. page ignored", "file", path, "error", err)
				continue
			}
			articles = append(articles, p)
		}

		for _, p := range articles {
			if slug == p.Meta.Slug {
				l.Debug("serving page",
					"path", p.Meta.Slug,
					"title", p.Meta.Title,
					"filename", p.filename,
					"html_length", len(p.Html))

				articleTmpl, err := baseTmpl.Clone()
				if err != nil {
					l.Error("clone base templates", "error", err)
					respondStatus(w, r, http.StatusInternalServerError)
					return
				}
				articleTmpl, err = articleTmpl.ParseFiles(tmplPageFolder + "/blogPost.html")
				if err != nil {
					l.Error("parse content template", "error", err)
					respondStatus(w, r, http.StatusInternalServerError)
					return
				}
				tmplData.Content = p
				err = articleTmpl.ExecuteTemplate(w, "layout", tmplData)
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
		pagePaths := make(map[string]string, len(articles))
		for _, p := range articles {
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

// Content Data Functions

func (h *handler) dataBlogIndex(globs ...string) func() any {
	// return type
	type blogIndexData struct {
		PostMetas []metadata
	}
	// setup
	if len(globs) == 0 {
		h.logger.Fatal("dataBlogIndex setup. no paths given")
	}
	// datafunction
	return func() any {
		// Find all markdown files
		var mdPaths []string
		for _, g := range globs {
			paths, err := filepath.Glob(g)
			if err != nil {
				h.logger.Fatal("glob markdown files", "error", err)
			}
			mdPaths = append(mdPaths, paths...)
		}
		h.logger.Debug("found markdown files", "files", mdPaths)

		md := goldmark.New(
			goldmark.WithExtensions(
				meta.Meta,
			),
		)

		var data blogIndexData

		for _, path := range mdPaths {
			p, err := preparePage(md, path)
			if err != nil {
				h.logger.Error("prepare page. page ignored", "file", path, "error", err)
				continue
			}
			data.PostMetas = append(data.PostMetas, p.Meta)
		}
		return data
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

	metaData, err := meta.TryGet(ctx)
	if err != nil {
		return article{}, fmt.Errorf("get metadata: %s", err)
	}

	pageMeta, err := parseMetadata(metaData)
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
		switch t := val.(type) {
		case string:
			if t == "" {
				return m, fmt.Errorf("title is empty")
			}
			m.Title = t
		default:
			return m, fmt.Errorf("title must be a string")
		}
	} else {
		return m, fmt.Errorf("no title found")
	}

	if val, ok := meta["slug"]; ok {
		switch t := val.(type) {
		case string:
			if t == "" {
				return m, fmt.Errorf("slug is empty")
			}
			m.Slug = t
		default:
			return m, fmt.Errorf("slug must be a string")
		}
	} else {
		return m, fmt.Errorf("no slug found")
	}

	if val, ok := meta["draft"]; ok {
		switch t := val.(type) {
		case bool:
			m.Draft = t
		default:
			return m, fmt.Errorf("draft must be a bool")
		}
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

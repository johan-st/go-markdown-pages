package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	chromaStyles "github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/log"
	"github.com/matryer/way"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"

	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	// folder containing templates
	tmplLayoutFolder = "templates/layout"
	tmplPageFolder   = "templates/pages"

	// base info for all pages
	mainNav = []navLink{
		{"/", "home", false},
		// {"/about", "about", false},
		// {"/projects", "projects", false},
		// {"/contact", "contact", false},
		{"/blog", "blog", false},
		{"/docs", "docs", false},
	}

	baseTitle = "docs"
	baseMeta  = map[string]string{"description": "TODO: add description", "keywords": "TODO: add keywords"}
)

type handler struct {
	logger *log.Logger
	router way.Router
}

type navLink struct {
	Url    string
	Name   string
	Active bool
}

type blogPost struct {
	Meta     metadata
	Html     string
	filename string
}

type metadata struct {
	Title       string
	Slug        string
	Draft       bool
	Date        time.Time
	Tags        []string
	Description string
	CssClass    string
}

type templateData struct {
	DocTitle string
	Nav      []navLink
	Meta     map[string]string

	Style   string
	Content any
}

func newHandler(l *log.Logger) *handler {
	h := &handler{
		logger: l,
		router: *way.NewRouter(),
	}

	return h
}

// ROUTES

// prepareRoutesDev prepares the routes for development. (dev: It reloads the templates on every request.)
func (h *handler) prepareRoutesDev() {
	h.router.HandleFunc("GET", "/git", h.handleGitWebhook())
	h.router.HandleFunc("GET", "/partials/:block", h.handlePartialsDev())
	h.router.HandleFunc("GET", "/public/", h.handlePublic())
	h.router.HandleFunc("GET", "/blog", h.handleTemplateDev("blog.html", h.dataBlogIndex(gitMdPath+"/*.md", "blog/*.md")))
	h.router.HandleFunc("GET", "/blog/:slug", h.handleMarkdownDev(gitMdPath+"/blog/*.md", "blog/*.md"))
	h.router.HandleFunc("GET", "/:slug", h.handleMarkdownDev(gitMdPath+"/pages/*.md", "pages/*.md"))

	// everything else
	h.router.HandleFunc("GET", "...", h.handleRedirect(http.StatusTemporaryRedirect, "/blog"))
	h.router.HandleFunc("*", "...", h.handleStatus(http.StatusMethodNotAllowed))
}

// HANDLERS

func (h *handler) handleStatus(status int) http.HandlerFunc {
	// setup
	l := h.logger.With("handler", "handleStatus")
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
				"status", status,
			)
		}(time.Now())

		respondStatus(w, r, status)
	}
}

func (h *handler) handleRedirect(status int, redirectPath string) http.HandlerFunc {
	// setup
	l := h.logger.With("handler", "handleRedirect")
	defer func(t time.Time) {
		l.Debug("handler ready", "time", time.Since(t))
	}(time.Now())

	if status < 300 || status > 399 {
		l.Error("invalid status code. Redirects might not work as expected",
			"status", status,
			"expected_status", "300 <= status <= 399",
		)
	}

	if redirectPath == "" {
		l.Fatal("redirect path is empty")
	}

	// handler
	return func(w http.ResponseWriter, r *http.Request) {
		// timer
		defer func(t time.Time) {
			l.Debug("responding",
				"time", time.Since(t),
				"request_path", r.URL.Path,
				"redirect_path", redirectPath,
				"status", status,
			)
		}(time.Now())

		if redirectPath == r.URL.Path {
			l.Debug("redirect loop detected",
				"request_path", r.URL.Path,
				"redirect_path", redirectPath,
				"status", status,
			)
			l.Error("redirect loop detected",
				"request_path", r.URL.Path,
				"redirect_path", redirectPath,
			)
			respondStatus(w, r, http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, redirectPath, status)
	}
}

func (h *handler) handleTemplateDev(contentTemplatePath string, getContentData func() any) http.HandlerFunc {
	// setup
	l := h.logger.With("handler", "handleContentDev")
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


		// find first path element
		firstPath := strings.Split(r.URL.Path, "/")[1]
		if firstPath == "" {
			firstPath = "/"
		}

		// set active navlink
		navlinks := mainNav
		for i := range navlinks {
			if navlinks[i].Url == fmt.Sprintf("/%s", firstPath) {
				navlinks[i].Active = true
				l.Debug("active navlink",
					"url", navlinks[i].Url,
					"name", navlinks[i].Name)
				continue
			}
			navlinks[i].Active = false
		}
		

		l.Debug("first path element",
			"path", firstPath)

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

		for _, tmpl := range contentTmpls.Templates() {
			l.Debug("template", "name", tmpl.Name())
		}

		tmplData := templateData{
			Meta:     baseMeta,
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
	l := h.logger.With("handler", "handleGitWebhook")
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

		requestedTemplate := way.Param(r.Context(), "block")

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

// handleMarkdownDev serves the blog. It populates with files matching '*.md' in the given directories. (dev: It reloads the templates on every request.)
func (h *handler) handleMarkdownDev(mdGlobs ...string) http.HandlerFunc {
	var (
		navLinks = mainNav
		tmplData = templateData{
			DocTitle: baseTitle,
			Nav:      navLinks,
			Meta:     baseMeta,
			Content:  nil,
		}

		defaultStyle    = "nord"
		defaultStyleInt = 0 // set in runtime
		randCodeStyle   = false
	)

	// setup
	l := h.logger.With("handler", "handlePage")
	defer func(t time.Time) {
		l.Debug("handler ready", "time", time.Since(t))
	}(time.Now())

	l.Debug("handleBlogDev setup", "globs", mdGlobs)
	if len(mdGlobs) == 0 {
		l.Fatal("handleBlogDev setup. no paths given")
	}

	highlightingStyles := chromaStyles.Names()
	if i, ok := indexOf(highlightingStyles, defaultStyle); ok {
		defaultStyleInt = i
	} else {
		l.Fatal("default highlighting style not found", "style", defaultStyle, "available_styles", highlightingStyles)
	}

	// handler
	return func(w http.ResponseWriter, r *http.Request) {

		defer func(t time.Time) {
			l.Info("response", "time", time.Since(t))
		}(time.Now())

		qStyle := strings.ToLower(r.URL.Query().Get("style"))

		reqStyleInt, ok := indexOf(highlightingStyles, qStyle)
		if ok {
			l.Debug("found highlighting style", "style", qStyle, "index", reqStyleInt)
		} else if randCodeStyle {
			l.Debug("highlighting style not found. using random", "style", qStyle)
			reqStyleInt = rand.Intn(len(highlightingStyles))
		} else {
			l.Debug("highlighting style not found. using default", "style", qStyle, "index", defaultStyleInt)
			reqStyleInt = defaultStyleInt
		}

		tmplData.Style = fmt.Sprintf("(%d/%d): %s", reqStyleInt+1, len(highlightingStyles), highlightingStyles[reqStyleInt])

		// setup markdown parser
		md := goldmark.New(
			goldmark.WithExtensions(
				highlighting.NewHighlighting(
					highlighting.WithStyle(highlightingStyles[reqStyleInt]),
				),
				extension.GFM,
				meta.Meta,
			),
			goldmark.WithRendererOptions(
				html.WithUnsafe(),
			),
		)

		slug := way.Param(r.Context(), "slug")
		l.Debug("handleBlogDev",
			"slug", slug,
		)

		// find first path element
		firstPath := strings.Split(r.URL.Path, "/")[1]
		if firstPath == "" {
			firstPath = "/"
		}

		// set active navlink
		navlinks := mainNav
		for i := range navlinks {
			if navlinks[i].Url == fmt.Sprintf("/%s", firstPath) {
				navlinks[i].Active = true
				continue
			}
			navlinks[i].Active = false
		}

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

		// prepare blogPosts
		var blogPosts []blogPost
		for _, path := range mdPaths {
			p, err := preparePage(md, path)
			if err != nil {
				l.Warn("prepare page. page ignored", "file", path, "reason", err)
				continue
			}
			blogPosts = append(blogPosts, p)
		}

		for _, p := range blogPosts {
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

				// prepare data
				tmplData.DocTitle = baseTitle + " | " + p.Meta.Title
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
		pagePaths := make(map[string]string, len(blogPosts))
		for _, p := range blogPosts {
			pagePaths[p.Meta.Slug] = p.Meta.Title
		}

		l.Debug("request path does not match any page",
			"request_path", requestPath,
			"response_code", http.StatusNotFound,
			"avaiable_paths", pagePaths,
		)
		h.handleRedirect(http.StatusTemporaryRedirect, "/404")(w, r)
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

func preparePage(md goldmark.Markdown, path string) (blogPost, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return blogPost{}, fmt.Errorf("read file: %s", err)
	}

	var html bytes.Buffer
	ctx := parser.NewContext()
	err = md.Convert(file, &html, parser.WithContext(ctx))
	if err != nil {
		return blogPost{}, fmt.Errorf("convert markdown: %s", err)
	}

	metaData, err := meta.TryGet(ctx)
	if err != nil {
		return blogPost{}, fmt.Errorf("get metadata: %s", err)
	}

	pageMeta, err := parseMetadata(metaData)
	if err != nil {
		return blogPost{}, fmt.Errorf("parse metadata: %s", err)
	}
	p := blogPost{
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
		case int:
			m.Slug = fmt.Sprintf("%d", t)
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

	if val, ok := meta["description"]; ok {
		switch t := val.(type) {
		case string:
			m.Description = t
		default:
			return m, fmt.Errorf("description must be a string")
		}
	}

	return m, nil
}

// OTHER

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

func indexOf(slice []string, s string) (int, bool) {
	for i, e := range slice {
		if e == s {
			return i, true
		}
	}
	return 0, false
}

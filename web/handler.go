package web

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"fabric-agents/auth"
	"fabric-agents/core"

	"github.com/gorilla/mux"
	"github.com/russross/blackfriday/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var templateFuncs = template.FuncMap{
	"markdown": func(text string) template.HTML {
		html := blackfriday.Run([]byte(text))
		return template.HTML(html)
	},
	"formatVideoTitle": func(title string) template.HTML {
		formattedTitle := cases.Title(language.English, cases.Compact).String(strings.ReplaceAll(title, "-", " "))
		return template.HTML(fmt.Sprintf("<span class='text-2xl font-bold text-indigo-400'>%s</span>", formattedTitle))
	},
}

type Handler struct {
	processor      *core.Processor
	router         *mux.Router
	dataDir        string
	logger         *slog.Logger
	authService    *auth.Service
	authEnabled    bool
	cookieSecure   bool
	envEditEnabled bool
}

func NewHandler(p *core.Processor, dataDir string, logger *slog.Logger, authService *auth.Service, authEnabled bool, cookieSecure bool, envEditEnabled bool) *Handler {
	h := &Handler{
		processor:      p,
		dataDir:        dataDir,
		logger:         logger,
		authService:    authService,
		authEnabled:    authEnabled,
		cookieSecure:   cookieSecure,
		envEditEnabled: envEditEnabled,
	}
	h.setupRoutes()
	h.logger.Info("Handler initialized")
	return h
}

func (h *Handler) setupRoutes() {
	h.router = mux.NewRouter()
	h.router.HandleFunc("/", h.handleLogin).Methods(http.MethodGet)
	h.router.HandleFunc("/login", h.handleLogin).Methods(http.MethodGet, http.MethodPost)
	h.router.HandleFunc("/logout", h.handleLogout).Methods(http.MethodPost, http.MethodGet)
	h.router.HandleFunc("/forbidden", h.handleForbidden).Methods(http.MethodGet)

	register := func(r *mux.Router) {
		r.HandleFunc("/", h.handleIndex).Methods(http.MethodGet)
		r.HandleFunc("/videos", h.handleVideos).Methods(http.MethodGet)
		r.HandleFunc("/videos/{id}", h.handleVideoByID).Methods(http.MethodGet, http.MethodDelete)
		r.HandleFunc("/videos/{id}/{summary}", h.handleVideoByIDSummary).Methods(http.MethodGet)

		r.HandleFunc("/submit-videos", h.handleSubmitVideos).Methods(http.MethodPost)
		r.HandleFunc("/process-video", h.handleProcessVideo).Methods(http.MethodPost)
		r.HandleFunc("/debug/{videoID}/{filename}", h.handleDebugFile).Methods(http.MethodGet)

		r.HandleFunc("/config", h.handleConfig).Methods(http.MethodGet)
		r.HandleFunc("/config/patterns", h.handleConfigPatterns).Methods(http.MethodGet)
		r.HandleFunc("/config/pattern/{name}", h.handleConfigPattern).Methods(http.MethodGet)
		r.HandleFunc("/config/pattern/{name}/save", h.handleSavePattern).Methods(http.MethodPost)
		if h.envEditEnabled {
			r.HandleFunc("/config/env", h.handleConfigEnv).Methods(http.MethodGet)
			r.HandleFunc("/config/env/save", h.handleSaveEnv).Methods(http.MethodPost)
		}
		r.HandleFunc("/config/models", h.handleConfigModels).Methods(http.MethodGet)
		r.HandleFunc("/config/models/save", h.handleSaveModels).Methods(http.MethodPost)
		r.HandleFunc("/config/models/refresh", h.handleRefreshModels).Methods(http.MethodPost)

		r.HandleFunc("/admin/users", h.handleAdminUsers).Methods(http.MethodGet)
		r.HandleFunc("/admin/users/new", h.handleAdminCreateUser).Methods(http.MethodPost)
		r.HandleFunc("/admin/users/{id}/role", h.handleAdminSetUserRole).Methods(http.MethodPost)
		r.HandleFunc("/admin/users/{id}/enabled", h.handleAdminSetUserEnabled).Methods(http.MethodPost)
	}

	if !h.authEnabled {
		register(h.router)
		return
	}

	authenticated := h.router.PathPrefix("/").Subrouter()
	authenticated.Use(auth.RequireAuth(h.authService, h.handleUnauthorized))

	viewer := authenticated.NewRoute().Subrouter()
	viewer.Use(auth.RequireRole(auth.RoleViewer, h.handleForbiddenResponse))
	viewer.HandleFunc("/home", h.handleIndex).Methods(http.MethodGet)
	viewer.HandleFunc("/videos", h.handleVideos).Methods(http.MethodGet)
	viewer.HandleFunc("/videos/{id}", h.handleVideoByID).Methods(http.MethodGet)
	viewer.HandleFunc("/videos/{id}/{summary}", h.handleVideoByIDSummary).Methods(http.MethodGet)

	operator := authenticated.NewRoute().Subrouter()
	operator.Use(auth.RequireRole(auth.RoleOperator, h.handleForbiddenResponse))
	operator.HandleFunc("/submit-videos", h.handleSubmitVideos).Methods(http.MethodPost)
	operator.HandleFunc("/process-video", h.handleProcessVideo).Methods(http.MethodPost)
	operator.HandleFunc("/videos/{id}", h.handleVideoByID).Methods(http.MethodDelete)
	operator.HandleFunc("/debug/{videoID}/{filename}", h.handleDebugFile).Methods(http.MethodGet)

	admin := authenticated.NewRoute().Subrouter()
	admin.Use(auth.RequireRole(auth.RoleAdmin, h.handleForbiddenResponse))
	admin.HandleFunc("/config", h.handleConfig).Methods(http.MethodGet)
	admin.HandleFunc("/config/patterns", h.handleConfigPatterns).Methods(http.MethodGet)
	admin.HandleFunc("/config/pattern/{name}", h.handleConfigPattern).Methods(http.MethodGet)
	admin.HandleFunc("/config/pattern/{name}/save", h.handleSavePattern).Methods(http.MethodPost)
	if h.envEditEnabled {
		admin.HandleFunc("/config/env", h.handleConfigEnv).Methods(http.MethodGet)
		admin.HandleFunc("/config/env/save", h.handleSaveEnv).Methods(http.MethodPost)
	}
	admin.HandleFunc("/config/models", h.handleConfigModels).Methods(http.MethodGet)
	admin.HandleFunc("/config/models/save", h.handleSaveModels).Methods(http.MethodPost)
	admin.HandleFunc("/config/models/refresh", h.handleRefreshModels).Methods(http.MethodPost)
	admin.HandleFunc("/admin/users", h.handleAdminUsers).Methods(http.MethodGet)
	admin.HandleFunc("/admin/users/new", h.handleAdminCreateUser).Methods(http.MethodPost)
	admin.HandleFunc("/admin/users/{id}/role", h.handleAdminSetUserRole).Methods(http.MethodPost)
	admin.HandleFunc("/admin/users/{id}/enabled", h.handleAdminSetUserEnabled).Methods(http.MethodPost)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

func forwardedPrefix(r *http.Request) string {
	prefix := strings.TrimSpace(r.Header.Get("X-Forwarded-Prefix"))
	if prefix == "" {
		return ""
	}

	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return ""
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return prefix
}

func prefixedPath(r *http.Request, path string) string {
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	prefix := forwardedPrefix(r)
	if prefix == "" {
		return path
	}

	return prefix + path
}

func templateData(r *http.Request, data map[string]interface{}) map[string]interface{} {
	if data == nil {
		data = map[string]interface{}{}
	}
	currentUser, isAuthenticated := auth.UserFromContext(r.Context())
	canOperate := false
	canAdmin := false
	if isAuthenticated {
		canOperate = auth.HasRequiredRole(currentUser.Role, auth.RoleOperator)
		canAdmin = auth.HasRequiredRole(currentUser.Role, auth.RoleAdmin)
	}
	if authEnabled, ok := data["AuthEnabled"].(bool); ok && !authEnabled {
		canOperate = true
		canAdmin = true
	}
	if authEnabled, ok := data["AuthEnabled"].(bool); !ok || !authEnabled {
		data["AuthEnabled"] = false
	} else {
		data["AuthEnabled"] = true
	}
	data["CurrentUser"] = currentUser
	data["IsAuthenticated"] = isAuthenticated
	data["CanOperate"] = canOperate
	data["CanAdmin"] = canAdmin
	data["BasePath"] = forwardedPrefix(r)
	return data
}

func isHTMX(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}

func (h *Handler) handleUnauthorized(w http.ResponseWriter, r *http.Request) {
	next := url.QueryEscape(r.URL.RequestURI())
	loginURL := prefixedPath(r, "/login?next="+next)
	h.logger.Warn("Unauthorized request", "method", r.Method, "path", r.URL.Path, "requestURI", r.URL.RequestURI(), "isHTMX", isHTMX(r), "redirect", loginURL)

	if isHTMX(r) {
		w.Header().Set("HX-Redirect", loginURL)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, loginURL, http.StatusFound)
}

func (h *Handler) handleForbiddenResponse(w http.ResponseWriter, r *http.Request) {
	if isHTMX(r) {
		h.logger.Warn("Forbidden request (HTMX)", "method", r.Method, "path", r.URL.Path, "requestURI", r.URL.RequestURI())
		w.Header().Set("HX-Redirect", prefixedPath(r, "/forbidden"))
		w.WriteHeader(http.StatusOK)
		return
	}
	h.logger.Warn("Forbidden request", "method", r.Method, "path", r.URL.Path, "requestURI", r.URL.RequestURI())
	http.Redirect(w, r, prefixedPath(r, "/forbidden"), http.StatusFound)
}

func (h *Handler) handleForbidden(w http.ResponseWriter, r *http.Request) {
	if h.authEnabled {
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil || cookie == nil {
			h.handleUnauthorized(w, r)
			return
		}
		if _, err := h.authService.ValidateSession(cookie.Value); err != nil {
			h.handleUnauthorized(w, r)
			return
		}
	}

	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/forbidden.html"))
	tmpl.Execute(w, templateData(r, map[string]interface{}{"Title": "Forbidden", "AuthEnabled": h.authEnabled}))
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !h.authEnabled {
		h.handleIndex(w, r)
		return
	}

	if r.Method == http.MethodGet {
		if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
			if _, err := h.authService.ValidateSession(cookie.Value); err == nil {
				http.Redirect(w, r, prefixedPath(r, "/home"), http.StatusFound)
				return
			}
		}

		next := r.URL.Query().Get("next")
		tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/login.html"))
		tmpl.Execute(w, templateData(r, map[string]interface{}{"Title": "Login", "Next": next, "AuthEnabled": h.authEnabled}))
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	next := strings.TrimSpace(r.FormValue("next"))
	if next == "" || !strings.HasPrefix(next, "/") || next == "/" {
		next = "/home"
	}

	user, err := h.authService.Authenticate(username, password)
	if err != nil {
		tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/login.html"))
		tmpl.Execute(w, templateData(r, map[string]interface{}{
			"Title":       "Login",
			"Error":       "Invalid username or password",
			"Next":        next,
			"AuthEnabled": h.authEnabled,
		}))
		return
	}

	sess, err := h.authService.CreateSession(user.ID)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    sess.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  sess.ExpiresAt,
	})

	http.Redirect(w, r, prefixedPath(r, next), http.StatusFound)
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
		_ = h.authService.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, prefixedPath(r, "/login"), http.StatusFound)
}

func (h *Handler) handleVideos(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling /videos request")
	videos, err := core.LoadVideos(h.dataDir)
	if err != nil {
		h.logger.Error("Failed to load videos", "error", err)
		http.Error(w, fmt.Sprintf("Failed to load videos: %v", err), http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/videos.html"))
	tmpl.Execute(w, templateData(r, map[string]interface{}{"Title": "Videos", "Videos": videos, "AuthEnabled": h.authEnabled}))
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling / request")
	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/index.html"))
	tmpl.Execute(w, templateData(r, map[string]interface{}{"Title": "Home", "AuthEnabled": h.authEnabled}))
}

func (h *Handler) handleSubmitVideos(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling /submit-videos request")
	videoLinks := r.FormValue("video_links")
	videoLinksList := strings.Split(videoLinks, "\n")
	for _, videoLink := range videoLinksList {
		h.logger.Info("Processing video link", "link", videoLink)
		h.processor.FetchVideo(videoLink)
	}
	h.logger.Info("Videos processed", "count", len(videoLinksList))
	fmt.Fprintf(w, "Videos processed: %d", len(videoLinksList))
}

func (h *Handler) handleVideoByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	videoID := vars["id"]
	h.logger.Debug("Handling /videos/{id} request", "videoID", videoID)

	// Check if this is a delete request
	if r.Method == "DELETE" {
		h.logger.Info("Deleting video", "videoID", videoID)
		err := core.DeleteVideo(videoID, h.dataDir)
		if err != nil {
			h.logger.Error("Failed to delete video", "videoID", videoID, "error", err)
			http.Error(w, fmt.Sprintf("Failed to delete video: %v", err), http.StatusInternalServerError)
			return
		}
		// Redirect to the videos list page after successful deletion
		w.Header().Set("HX-Redirect", prefixedPath(r, "/videos"))
		return
	}

	video, err := core.LoadVideo(videoID, h.dataDir)
	if err != nil {
		h.logger.Error("Failed to load video", "videoID", videoID, "error", err)
		http.Error(w, fmt.Sprintf("Failed to load video: %v", err), http.StatusInternalServerError)
		return
	}
	files, err := core.LoadVideoFiles(videoID, h.dataDir)
	if err != nil {
		h.logger.Error("Failed to load video files", "videoID", videoID, "error", err)
		http.Error(w, fmt.Sprintf("Failed to load video files: %v", err), http.StatusInternalServerError)
		return
	}
	models, err := core.LoadModels()
	if err != nil {
		h.logger.Warn("Failed to load configured models file; falling back to dynamic configured providers", "error", err)
		models, err = core.ListConfiguredModels()
		if err != nil {
			h.logger.Warn("Failed to load dynamic configured models; falling back to default-only", "error", err)
			models = []core.Model{}
		}
	} else {
		models, err = core.FilterModelsByConfiguredProviders(models)
		if err != nil {
			h.logger.Warn("Failed to filter configured models by provider keys; falling back to default-only", "error", err)
			models = []core.Model{}
		}
	}
	patterns, err := core.ListPatterns()
	if err != nil {
		h.logger.Error("Failed to load patterns", "error", err)
		http.Error(w, fmt.Sprintf("Failed to load patterns: %v", err), http.StatusInternalServerError)
		return
	}
	savedPatterns, err := core.LoadPatterns()
	if err != nil {
		h.logger.Error("Failed to load saved patterns", "error", err)
		http.Error(w, fmt.Sprintf("Failed to load patterns: %v", err), http.StatusInternalServerError)
		return
	}
	tmpl, err := template.ParseFiles("web/templates/layout.html", "web/templates/video.html")
	if err != nil {
		h.logger.Error("Failed to parse template", "error", err)
		http.Error(w, fmt.Sprintf("Failed to parse template: %v", err), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, templateData(r, map[string]interface{}{
		"Title":       "Video",
		"VideoID":     videoID,
		"VideoTitle":  video.Title,
		"Files":       files,
		"Models":      models,
		"Patterns":    savedPatterns,
		"AllPatterns": patterns,
		"AuthEnabled": h.authEnabled,
	}))
	if err != nil {
		h.logger.Error("Failed to execute template", "error", err)
		http.Error(w, fmt.Sprintf("Failed to execute template: %v", err), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) handleVideoByIDSummary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	videoID := vars["id"]
	summary := vars["summary"]
	h.logger.Debug("Handling /videos/{id}/{summary} request", "videoID", videoID, "summary", summary)

	summary, err := core.LoadVideoSummary(videoID, h.dataDir, summary)
	if err != nil {
		h.logger.Error("Failed to load video summary", "videoID", videoID, "summary", summary, "error", err)
		http.Error(w, fmt.Sprintf("Failed to load video summary: %v", err), http.StatusInternalServerError)
		return
	}

	tmpl, err := template.New("layout.html").Funcs(templateFuncs).ParseFiles("web/templates/layout.html", "web/templates/video-summary.html")
	if err != nil {
		h.logger.Error("Failed to parse template", "error", err)
		http.Error(w, fmt.Sprintf("Failed to parse template: %v", err), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, templateData(r, map[string]interface{}{"Title": "Video", "VideoID": videoID, "Summary": summary, "AuthEnabled": h.authEnabled}))
}

func (h *Handler) handleProcessVideo(w http.ResponseWriter, r *http.Request) {
	videoID := r.FormValue("videoID")
	model := r.FormValue("model")
	pattern := r.FormValue("pattern")
	h.logger.Debug("Handling /process-video request", "videoID", videoID, "model", model, "pattern", pattern)

	_, _, traceFileName, err := h.processor.ProcessVideo(videoID, model, pattern)
	if err != nil {
		h.logger.Error("Failed to process video", "videoID", videoID, "model", model, "pattern", pattern, "error", err)

		if traceFileName != "" {
			traceLink := prefixedPath(r, fmt.Sprintf("/videos/%s/%s", videoID, url.PathEscape(traceFileName)))
			fmt.Fprintf(w, `<li><a href="%s" class="text-indigo-400 hover:text-indigo-300 transition duration-150 ease-in-out">%s</a></li>`, traceLink, traceFileName)
		}

		fmt.Fprintf(w, `<li class="text-red-500">Pattern execution failed: %s</li>`, template.HTMLEscapeString(err.Error()))
		return
	}
	videoLink := prefixedPath(r, fmt.Sprintf("/videos/%s/%s-%s.md", videoID, url.PathEscape(pattern), url.PathEscape(model)))
	fmt.Fprintf(w, `<li><a href="%s" class="text-indigo-400 hover:text-indigo-300 transition duration-150 ease-in-out">%s-%s.md</a></li>`, videoLink, pattern, model)

	if traceFileName != "" {
		traceLink := prefixedPath(r, fmt.Sprintf("/videos/%s/%s", videoID, url.PathEscape(traceFileName)))
		fmt.Fprintf(w, `<li><a href="%s" class="text-indigo-400 hover:text-indigo-300 transition duration-150 ease-in-out">%s</a></li>`, traceLink, traceFileName)
	}
}

func (h *Handler) handleDebugFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	videoID := vars["videoID"]
	filename := vars["filename"]
	h.logger.Debug("Handling /debug/{videoID}/{filename} request", "videoID", videoID, "filename", filename)

	// Construct the file path
	filePath := filepath.Join("data", "videos", videoID, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		h.logger.Error("Debug file not found", "videoID", videoID, "filename", filename, "path", filePath)
		http.Error(w, fmt.Sprintf("Debug file not found: %s", filename), http.StatusNotFound)
		return
	}

	// Read and serve the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		h.logger.Error("Failed to read debug file", "videoID", videoID, "filename", filename, "error", err)
		http.Error(w, fmt.Sprintf("Failed to read debug file: %v", err), http.StatusInternalServerError)
		return
	}

	// Set appropriate content type based on file extension
	if strings.HasSuffix(filename, ".xml") {
		w.Header().Set("Content-Type", "application/xml")
	} else if strings.HasSuffix(filename, ".txt") {
		w.Header().Set("Content-Type", "text/plain")
	} else {
		w.Header().Set("Content-Type", "text/plain")
	}

	// Write the file content
	w.Write(content)
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling /config request")
	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/config.html"))
	tmpl.Execute(w, templateData(r, map[string]interface{}{"Title": "Configuration", "AuthEnabled": h.authEnabled, "EnvEditEnabled": h.envEditEnabled}))
}

func (h *Handler) handleConfigPatterns(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling /config/patterns request")
	patterns, err := core.ListPatterns()
	if err != nil {
		h.logger.Error("Failed to load patterns", "error", err)
		http.Error(w, fmt.Sprintf("Failed to load patterns: %v", err), http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/config-patterns.html"))
	tmpl.Execute(w, templateData(r, map[string]interface{}{"Title": "Edit Patterns", "Patterns": patterns, "AuthEnabled": h.authEnabled}))
}

func (h *Handler) handleConfigPattern(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	patternName := vars["name"]
	h.logger.Debug("Handling /config/pattern/{name} request", "pattern", patternName)

	// Get the fabric config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		h.logger.Error("Failed to get home directory", "error", err)
		http.Error(w, fmt.Sprintf("Failed to get home directory: %v", err), http.StatusInternalServerError)
		return
	}

	patternPath := filepath.Join(homeDir, ".config", "fabric", "patterns", patternName, "system.md")
	content, err := os.ReadFile(patternPath)
	if err != nil {
		h.logger.Error("Failed to read pattern file", "pattern", patternName, "error", err)
		http.Error(w, fmt.Sprintf("Failed to read pattern file: %v", err), http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/config-pattern-edit.html"))
	tmpl.Execute(w, templateData(r, map[string]interface{}{
		"Title":       "Edit Pattern",
		"PatternName": patternName,
		"Content":     string(content),
		"AuthEnabled": h.authEnabled,
	}))
}

func (h *Handler) handleSavePattern(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	patternName := vars["name"]
	h.logger.Debug("Handling /config/pattern/{name}/save request", "pattern", patternName)

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	content := r.FormValue("content")

	// Get the fabric config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		h.logger.Error("Failed to get home directory", "error", err)
		http.Error(w, fmt.Sprintf("Failed to get home directory: %v", err), http.StatusInternalServerError)
		return
	}

	patternPath := filepath.Join(homeDir, ".config", "fabric", "patterns", patternName, "system.md")
	err = os.WriteFile(patternPath, []byte(content), 0644)
	if err != nil {
		h.logger.Error("Failed to save pattern file", "pattern", patternName, "error", err)
		http.Error(w, fmt.Sprintf("Failed to save pattern file: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Pattern saved successfully", "pattern", patternName)
	w.Header().Set("HX-Redirect", prefixedPath(r, "/config/patterns"))
	fmt.Fprintf(w, "Pattern saved successfully")
}

func (h *Handler) handleConfigEnv(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling /config/env request")
	if !h.envEditEnabled {
		http.NotFound(w, r)
		return
	}

	// Get the fabric config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		h.logger.Error("Failed to get home directory", "error", err)
		http.Error(w, fmt.Sprintf("Failed to get home directory: %v", err), http.StatusInternalServerError)
		return
	}

	envPath := filepath.Join(homeDir, ".config", "fabric", ".env")
	content, err := os.ReadFile(envPath)
	if err != nil {
		h.logger.Error("Failed to read .env file", "error", err)
		http.Error(w, fmt.Sprintf("Failed to read .env file: %v", err), http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/config-env.html"))
	tmpl.Execute(w, templateData(r, map[string]interface{}{
		"Title":       "Edit .env File",
		"Content":     string(content),
		"AuthEnabled": h.authEnabled,
	}))
}

func (h *Handler) handleSaveEnv(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling /config/env/save request")
	if !h.envEditEnabled {
		http.NotFound(w, r)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	content := r.FormValue("content")

	// Get the fabric config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		h.logger.Error("Failed to get home directory", "error", err)
		http.Error(w, fmt.Sprintf("Failed to get home directory: %v", err), http.StatusInternalServerError)
		return
	}

	envPath := filepath.Join(homeDir, ".config", "fabric", ".env")
	err = os.WriteFile(envPath, []byte(content), 0644)
	if err != nil {
		h.logger.Error("Failed to save .env file", "error", err)
		http.Error(w, fmt.Sprintf("Failed to save .env file: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info(".env file saved successfully")
	w.Header().Set("HX-Redirect", prefixedPath(r, "/config"))
	fmt.Fprintf(w, ".env file saved successfully")
}

func (h *Handler) handleConfigModels(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling /config/models request")

	models, err := core.LoadModelNames()
	if err != nil {
		h.logger.Error("Failed to load models list", "error", err)
		http.Error(w, fmt.Sprintf("Failed to load models list: %v", err), http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/config-models.html"))
	tmpl.Execute(w, templateData(r, map[string]interface{}{
		"Title":         "Edit Models",
		"ModelsContent": strings.Join(models, "\n"),
		"ModelCount":    len(models),
		"AuthEnabled":   h.authEnabled,
	}))
}

func (h *Handler) handleSaveModels(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling /config/models/save request")

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	content := r.FormValue("content")
	modelLines := strings.Split(content, "\n")

	err := core.SaveModelNames(modelLines)
	if err != nil {
		h.logger.Error("Failed to save models list", "error", err)
		http.Error(w, fmt.Sprintf("Failed to save models list: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Models list saved successfully")
	w.Header().Set("HX-Redirect", prefixedPath(r, "/config/models"))
	fmt.Fprintf(w, "Models list saved successfully")
}

func (h *Handler) handleRefreshModels(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling /config/models/refresh request")

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	models, err := core.RefreshModelNamesFromConfiguredProviders()
	if err != nil {
		h.logger.Error("Failed to refresh models list", "error", err)
		http.Error(w, fmt.Sprintf("Failed to refresh models list: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Models list refreshed successfully", "count", len(models))
	w.Header().Set("HX-Redirect", prefixedPath(r, "/config/models"))
	fmt.Fprintf(w, "Models list refreshed successfully (%d models)", len(models))
}

func (h *Handler) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.authService.ListUsers()
	if err != nil {
		h.logger.Error("Failed to list users", "error", err)
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/admin-users.html"))
	tmpl.Execute(w, templateData(r, map[string]interface{}{
		"Title":       "User Administration",
		"Users":       users,
		"AuthEnabled": h.authEnabled,
	}))
}

func (h *Handler) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	role := auth.Role(strings.TrimSpace(r.FormValue("role")))

	if err := h.authService.CreateUser(username, password, role); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create user: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", prefixedPath(r, "/admin/users"))
	fmt.Fprintf(w, "User created")
}

func (h *Handler) handleAdminSetUserRole(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["id"]
	role := auth.Role(strings.TrimSpace(r.FormValue("role")))

	if err := h.authService.SetUserRole(userID, role); err != nil {
		http.Error(w, fmt.Sprintf("Failed to set role: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.authService.EnsureAtLeastOneAdmin(); err != nil {
		_ = h.authService.SetUserRole(userID, auth.RoleViewer)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", prefixedPath(r, "/admin/users"))
	fmt.Fprintf(w, "Role updated")
}

func (h *Handler) handleAdminSetUserEnabled(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["id"]
	enabled := strings.EqualFold(strings.TrimSpace(r.FormValue("enabled")), "true")

	if err := h.authService.SetUserEnabled(userID, enabled); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update user status: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.authService.EnsureAtLeastOneAdmin(); err != nil {
		_ = h.authService.SetUserEnabled(userID, true)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", prefixedPath(r, "/admin/users"))
	fmt.Fprintf(w, "User status updated")
}

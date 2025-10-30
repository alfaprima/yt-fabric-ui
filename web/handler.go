package web

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	processor *core.Processor
	router    *mux.Router
	dataDir   string
	logger    *slog.Logger
}

func NewHandler(p *core.Processor, dataDir string, logger *slog.Logger) *Handler {
	h := &Handler{
		processor: p,
		dataDir:   dataDir,
		logger:    logger,
	}
	h.setupRoutes()
	h.logger.Info("Handler initialized")
	return h
}

func (h *Handler) setupRoutes() {
	h.router = mux.NewRouter()
	h.router.HandleFunc("/", h.handleIndex)
	h.router.HandleFunc("/submit-videos", h.handleSubmitVideos)
	h.router.HandleFunc("/videos", h.handleVideos)
	h.router.HandleFunc("/process-video", h.handleProcessVideo)
	h.router.HandleFunc("/videos/{id}", h.handleVideoByID)
	h.router.HandleFunc("/videos/{id}/{summary}", h.handleVideoByIDSummary)
	h.router.HandleFunc("/debug/{videoID}/{filename}", h.handleDebugFile)
	h.router.HandleFunc("/config", h.handleConfig)
	h.router.HandleFunc("/config/patterns", h.handleConfigPatterns)
	h.router.HandleFunc("/config/pattern/{name}", h.handleConfigPattern)
	h.router.HandleFunc("/config/pattern/{name}/save", h.handleSavePattern)
	h.router.HandleFunc("/config/env", h.handleConfigEnv)
	h.router.HandleFunc("/config/env/save", h.handleSaveEnv)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
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
	tmpl.Execute(w, map[string]interface{}{"Title": "Videos", "Videos": videos})
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling / request")
	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/index.html"))
	tmpl.Execute(w, map[string]string{"Title": "Home"})
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
		w.Header().Set("HX-Redirect", "/videos")
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
	models, err := core.ListModels()
	if err != nil {
		h.logger.Error("Failed to load models", "error", err)
		http.Error(w, fmt.Sprintf("Failed to load models: %v", err), http.StatusInternalServerError)
		return
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
	savedModels, err := core.LoadModels()
	if err != nil {
		h.logger.Error("Failed to load saved models", "error", err)
		http.Error(w, fmt.Sprintf("Failed to load models: %v", err), http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("web/templates/layout.html", "web/templates/video.html")
	if err != nil {
		h.logger.Error("Failed to parse template", "error", err)
		http.Error(w, fmt.Sprintf("Failed to parse template: %v", err), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, map[string]interface{}{
		"Title":       "Video",
		"VideoID":     videoID,
		"VideoTitle":  video.Title,
		"Files":       files,
		"Models":      savedModels,
		"Patterns":    savedPatterns,
		"AllModels":   models,
		"AllPatterns": patterns,
	})
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
	tmpl.Execute(w, map[string]string{"Title": "Video", "VideoID": videoID, "Summary": summary})
}

func (h *Handler) handleProcessVideo(w http.ResponseWriter, r *http.Request) {
	videoID := r.FormValue("videoID")
	model := r.FormValue("model")
	pattern := r.FormValue("pattern")
	h.logger.Debug("Handling /process-video request", "videoID", videoID, "model", model, "pattern", pattern)

	_, _, err := h.processor.ProcessVideo(videoID, model, pattern)
	if err != nil {
		h.logger.Error("Failed to process video", "videoID", videoID, "model", model, "pattern", pattern, "error", err)
		http.Error(w, fmt.Sprintf("Failed to process video: %v", err), http.StatusInternalServerError)
		return
	}
	videoLink := fmt.Sprintf("/videos/%s/%s-%s.md", videoID, pattern, model)
	fmt.Fprintf(w, `<li><a href="%s" class="text-indigo-400 hover:text-indigo-300 transition duration-150 ease-in-out">%s-%s.md</a></li>`, videoLink, pattern, model)
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
	tmpl.Execute(w, map[string]string{"Title": "Configuration"})
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
	tmpl.Execute(w, map[string]interface{}{"Title": "Edit Patterns", "Patterns": patterns})
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
	tmpl.Execute(w, map[string]interface{}{
		"Title":       "Edit Pattern",
		"PatternName": patternName,
		"Content":     string(content),
	})
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
	w.Header().Set("HX-Redirect", "/config/patterns")
	fmt.Fprintf(w, "Pattern saved successfully")
}

func (h *Handler) handleConfigEnv(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling /config/env request")

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
	tmpl.Execute(w, map[string]interface{}{
		"Title":   "Edit .env File",
		"Content": string(content),
	})
}

func (h *Handler) handleSaveEnv(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling /config/env/save request")

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
	w.Header().Set("HX-Redirect", "/config")
	fmt.Fprintf(w, ".env file saved successfully")
}

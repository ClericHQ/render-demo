package handlers

import (
	_ "embed"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shahram/prompt-registry/backend/models"
	"github.com/shahram/prompt-registry/backend/store"
)

//go:embed frontend.html
var frontendHTML []byte

// Handler holds dependencies for HTTP handlers
type Handler struct {
	Store   store.Store
	Logger  *slog.Logger
	Metrics *Metrics
}

// New creates a new Handler with initialized metrics
func New(s store.Store, logger *slog.Logger) *Handler {
	return &Handler{
		Store:   s,
		Logger:  logger,
		Metrics: NewMetrics(),
	}
}

// Routes sets up all HTTP routes with middleware
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("POST /api/prompts", h.handleCreatePrompt)
	mux.HandleFunc("GET /api/prompts", h.handleListPrompts)
	mux.HandleFunc("GET /api/prompts/{slug}", h.handleGetPrompt)
	mux.HandleFunc("GET /api/prompts/{slug}/versions", h.handleListVersions)
	mux.HandleFunc("POST /api/prompts/{slug}/versions", h.handleCreateVersion)
	mux.HandleFunc("GET /api/prompts/{slug}/versions/{version}", h.handleGetVersion)

	// System routes
	mux.HandleFunc("GET /health", h.handleHealth)
	mux.HandleFunc("GET /metrics", h.handleMetrics)

	// Catch-all: Serve frontend for all other GET requests (client-side routing)
	mux.HandleFunc("GET /", h.handleFrontend)

	// Apply middleware
	var handler http.Handler = mux
	handler = h.corsMiddleware(handler)
	handler = h.loggingMiddleware(handler)
	handler = h.recoverMiddleware(handler)

	return handler
}

// Middleware: Panic recovery
func (h *Handler) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				h.Logger.Error("panic recovered",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
				)
				h.Metrics.IncrementHTTPErrors()
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Middleware: Request logging
func (h *Handler) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.Metrics.IncrementHTTPRequests()

		// Wrap ResponseWriter to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		h.Logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", duration.Milliseconds(),
		)
	})
}

// Middleware: CORS
func (h *Handler) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Handler: Create prompt
func (h *Handler) handleCreatePrompt(w http.ResponseWriter, r *http.Request) {
	var input models.CreatePromptInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.Logger.Error("failed to decode request", "error", err)
		h.respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	result, err := h.Store.CreatePrompt(input)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			h.respondError(w, http.StatusConflict, err.Error())
			return
		}
		if strings.Contains(err.Error(), "cannot be empty") {
			h.respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.Logger.Error("failed to create prompt", "error", err)
		h.respondError(w, http.StatusInternalServerError, "Failed to create prompt")
		return
	}

	h.Metrics.IncrementPromptsCreated()
	h.Metrics.IncrementPromptVersionsCreated()
	h.respondJSON(w, http.StatusCreated, result)
}

// Handler: List prompts
func (h *Handler) handleListPrompts(w http.ResponseWriter, r *http.Request) {
	limit := 100
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil {
			limit = val
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if val, err := strconv.Atoi(offsetStr); err == nil {
			offset = val
		}
	}

	results, err := h.Store.ListPrompts(limit, offset)
	if err != nil {
		h.Logger.Error("failed to list prompts", "error", err)
		h.respondError(w, http.StatusInternalServerError, "Failed to list prompts")
		return
	}

	h.respondJSON(w, http.StatusOK, results)
}

// Handler: Get prompt by slug
func (h *Handler) handleGetPrompt(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	result, err := h.Store.GetPromptBySlug(slug)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		h.Logger.Error("failed to get prompt", "error", err, "slug", slug)
		h.respondError(w, http.StatusInternalServerError, "Failed to get prompt")
		return
	}

	h.respondJSON(w, http.StatusOK, result)
}

// Handler: List versions
func (h *Handler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	results, err := h.Store.ListPromptVersions(slug)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		h.Logger.Error("failed to list versions", "error", err, "slug", slug)
		h.respondError(w, http.StatusInternalServerError, "Failed to list versions")
		return
	}

	h.respondJSON(w, http.StatusOK, results)
}

// Handler: Create version
func (h *Handler) handleCreateVersion(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	var input models.CreatePromptVersionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.Logger.Error("failed to decode request", "error", err)
		h.respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	result, err := h.Store.CreatePromptVersion(slug, input)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		if strings.Contains(err.Error(), "cannot be empty") {
			h.respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.Logger.Error("failed to create version", "error", err, "slug", slug)
		h.respondError(w, http.StatusInternalServerError, "Failed to create version")
		return
	}

	h.Metrics.IncrementPromptVersionsCreated()
	h.respondJSON(w, http.StatusCreated, result)
}

// Handler: Get specific version
func (h *Handler) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	versionStr := r.PathValue("version")

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid version number")
		return
	}

	result, err := h.Store.GetPromptVersion(slug, version)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		h.Logger.Error("failed to get version", "error", err, "slug", slug, "version", version)
		h.respondError(w, http.StatusInternalServerError, "Failed to get version")
		return
	}

	h.respondJSON(w, http.StatusOK, result)
}

// Handler: Health check
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status":   "healthy",
		"database": "connected",
	}

	// Verify database connectivity
	if _, err := h.Store.GetStats(); err != nil {
		h.Logger.Error("health check failed", "error", err)
		response["database"] = "error"
		h.respondJSON(w, http.StatusInternalServerError, response)
		return
	}

	h.respondJSON(w, http.StatusOK, response)
}

// Handler: Metrics
func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(h.Metrics.ExportPrometheus()))
}

// Helper: Respond with JSON
func (h *Handler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.Logger.Error("failed to encode response", "error", err)
		h.Metrics.IncrementHTTPErrors()
	}
}

// Helper: Respond with error
func (h *Handler) respondError(w http.ResponseWriter, status int, message string) {
	h.Metrics.IncrementHTTPErrors()
	h.respondJSON(w, status, map[string]string{"error": message})
}

// ErrorResponse wraps error messages
type ErrorResponse struct {
	Error string `json:"error"`
}

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
)

// Handler: Serve frontend
func (h *Handler) handleFrontend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(frontendHTML)
}

package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/shahram/prompt-registry/backend/store"
)

func setupTestHandler(t *testing.T) *Handler {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return New(s, logger)
}

// Test POST /api/prompts
func TestCreatePromptHandler_Success(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	body := `{
		"title": "Test Prompt",
		"description": "Test Description",
		"content": "Test Content"
	}`

	req := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["title"] != "Test Prompt" {
		t.Errorf("Expected title 'Test Prompt', got %v", response["title"])
	}

	// Verify current_version exists and has version_number 1
	currentVersion, ok := response["current_version"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected current_version to be an object")
	}
	if currentVersion["version_number"] != float64(1) {
		t.Errorf("Expected version_number 1, got %v", currentVersion["version_number"])
	}
	if currentVersion["content"] != "Test Content" {
		t.Errorf("Expected content 'Test Content', got %v", currentVersion["content"])
	}
}

func TestCreatePromptHandler_EmptyTitle(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	body := `{"title": "", "content": "Test Content"}`
	req := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestCreatePromptHandler_EmptyContent(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	body := `{"title": "Test", "content": ""}`
	req := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestCreatePromptHandler_DuplicateSlug(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	// Create first prompt
	body := `{"slug": "test-slug", "title": "Test 1", "content": "Content 1"}`
	req := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("First prompt creation failed with status %d", w.Code)
	}

	// Try to create second with same slug
	body2 := `{"slug": "test-slug", "title": "Test 2", "content": "Content 2"}`
	req2 := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", w2.Code)
	}
}

func TestCreatePromptHandler_MalformedJSON(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	body := `{"title": "Test", invalid json}`
	req := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// Test GET /api/prompts
func TestListPromptsHandler_Success(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	// Create some prompts first
	for i := 1; i <= 3; i++ {
		body := `{"title": "Prompt ` + string(rune('0'+i)) + `", "content": "Content"}`
		req := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	// List prompts
	req := httptest.NewRequest("GET", "/api/prompts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response) != 3 {
		t.Errorf("Expected 3 prompts, got %d", len(response))
	}

	// Verify structure
	if _, ok := response[0]["slug"]; !ok {
		t.Error("Expected slug field in response")
	}
	if _, ok := response[0]["title"]; !ok {
		t.Error("Expected title field in response")
	}
	if _, ok := response[0]["current_version"]; !ok {
		t.Error("Expected current_version field in response")
	}
}

func TestListPromptsHandler_Pagination(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	// Create 5 prompts
	for i := 1; i <= 5; i++ {
		body := `{"title": "Prompt ` + string(rune('0'+i)) + `", "content": "Content"}`
		req := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	// Get first 2
	req := httptest.NewRequest("GET", "/api/prompts?limit=2&offset=0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response) != 2 {
		t.Errorf("Expected 2 prompts, got %d", len(response))
	}
}

// Test GET /api/prompts/{slug}
func TestGetPromptHandler_Success(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	// Create a prompt
	body := `{"slug": "test-prompt", "title": "Test Prompt", "description": "Test Desc", "content": "Test Content"}`
	req := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Get the prompt
	req2 := httptest.NewRequest("GET", "/api/prompts/test-prompt", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w2.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["slug"] != "test-prompt" {
		t.Errorf("Expected slug 'test-prompt', got %v", response["slug"])
	}
	if response["title"] != "Test Prompt" {
		t.Errorf("Expected title 'Test Prompt', got %v", response["title"])
	}
	if response["description"] != "Test Desc" {
		t.Errorf("Expected description 'Test Desc', got %v", response["description"])
	}

	// Verify current_version
	currentVersion, ok := response["current_version"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected current_version to be an object")
	}
	if currentVersion["content"] != "Test Content" {
		t.Errorf("Expected content 'Test Content', got %v", currentVersion["content"])
	}
}

func TestGetPromptHandler_NotFound(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	req := httptest.NewRequest("GET", "/api/prompts/non-existent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// Test GET /api/prompts/{slug}/versions
func TestListVersionsHandler_Success(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	// Create a prompt
	body := `{"slug": "test-prompt", "title": "Test Prompt", "content": "Version 1"}`
	req := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Create version 2
	body2 := `{"content": "Version 2"}`
	req2 := httptest.NewRequest("POST", "/api/prompts/test-prompt/versions", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	// List versions
	req3 := httptest.NewRequest("GET", "/api/prompts/test-prompt/versions", nil)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w3.Code)
	}

	var response []map[string]interface{}
	if err := json.NewDecoder(w3.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response) != 2 {
		t.Errorf("Expected 2 versions, got %d", len(response))
	}

	// Verify structure
	if response[0]["version_number"] != float64(1) {
		t.Errorf("Expected first version to be 1, got %v", response[0]["version_number"])
	}
	if response[1]["version_number"] != float64(2) {
		t.Errorf("Expected second version to be 2, got %v", response[1]["version_number"])
	}
}

func TestListVersionsHandler_NotFound(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	req := httptest.NewRequest("GET", "/api/prompts/non-existent/versions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// Test POST /api/prompts/{slug}/versions
func TestCreateVersionHandler_Success(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	// Create a prompt
	body := `{"slug": "test-prompt", "title": "Test Prompt", "content": "Version 1"}`
	req := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Create version 2
	body2 := `{"content": "Version 2"}`
	req2 := httptest.NewRequest("POST", "/api/prompts/test-prompt/versions", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w2.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w2.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify current_version is now 2
	currentVersion, ok := response["current_version"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected current_version to be an object")
	}
	if currentVersion["version_number"] != float64(2) {
		t.Errorf("Expected version_number 2, got %v", currentVersion["version_number"])
	}
	if currentVersion["content"] != "Version 2" {
		t.Errorf("Expected content 'Version 2', got %v", currentVersion["content"])
	}
}

func TestCreateVersionHandler_EmptyContent(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	// Create a prompt
	body := `{"slug": "test-prompt", "title": "Test Prompt", "content": "Version 1"}`
	req := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Try to create version with empty content
	body2 := `{"content": ""}`
	req2 := httptest.NewRequest("POST", "/api/prompts/test-prompt/versions", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w2.Code)
	}
}

func TestCreateVersionHandler_NotFound(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	body := `{"content": "Version 1"}`
	req := httptest.NewRequest("POST", "/api/prompts/non-existent/versions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// Test GET /api/prompts/{slug}/versions/{version}
func TestGetVersionHandler_Success(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	// Create a prompt
	body := `{"slug": "test-prompt", "title": "Test Prompt", "content": "Version 1"}`
	req := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Create version 2
	body2 := `{"content": "Version 2"}`
	req2 := httptest.NewRequest("POST", "/api/prompts/test-prompt/versions", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	// Get version 1
	req3 := httptest.NewRequest("GET", "/api/prompts/test-prompt/versions/1", nil)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w3.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w3.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["version_number"] != float64(1) {
		t.Errorf("Expected version_number 1, got %v", response["version_number"])
	}
	if response["content"] != "Version 1" {
		t.Errorf("Expected content 'Version 1', got %v", response["content"])
	}

	// Get version 2
	req4 := httptest.NewRequest("GET", "/api/prompts/test-prompt/versions/2", nil)
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w4.Code)
	}

	var response2 map[string]interface{}
	if err := json.NewDecoder(w4.Body).Decode(&response2); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response2["version_number"] != float64(2) {
		t.Errorf("Expected version_number 2, got %v", response2["version_number"])
	}
	if response2["content"] != "Version 2" {
		t.Errorf("Expected content 'Version 2', got %v", response2["content"])
	}
}

func TestGetVersionHandler_NotFound(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	// Create a prompt with only version 1
	body := `{"slug": "test-prompt", "title": "Test Prompt", "content": "Version 1"}`
	req := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Try to get non-existent version
	req2 := httptest.NewRequest("GET", "/api/prompts/test-prompt/versions/99", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w2.Code)
	}
}

// Test GET /health
func TestHealthHandler_Healthy(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", response["status"])
	}
	if response["database"] != "connected" {
		t.Errorf("Expected database 'connected', got %v", response["database"])
	}
}

// Test GET /metrics
func TestMetricsHandler_Success(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	// Create a prompt to increment metrics
	body := `{"title": "Test Prompt", "content": "Test Content"}`
	req := httptest.NewRequest("POST", "/api/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Get metrics
	req2 := httptest.NewRequest("GET", "/metrics", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}

	body2 := w2.Body.String()

	// Verify metrics exist
	expectedMetrics := []string{
		"prompts_created_total",
		"prompt_versions_created_total",
		"http_requests_total",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(body2, metric) {
			t.Errorf("Expected metrics to contain %q", metric)
		}
	}
}

// Test CORS headers
func TestCORSHeaders(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	req := httptest.NewRequest("GET", "/api/prompts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected CORS header '*', got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSOptions(t *testing.T) {
	h := setupTestHandler(t)
	router := h.Routes()

	req := httptest.NewRequest("OPTIONS", "/api/prompts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected CORS origin header '*', got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Headers") != "Content-Type" {
		t.Errorf("Expected CORS headers 'Content-Type', got %q", w.Header().Get("Access-Control-Allow-Headers"))
	}
}

// Test panic recovery
func TestPanicRecovery(t *testing.T) {
	h := setupTestHandler(t)

	// Create a handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Wrap with recovery middleware
	wrapped := h.recoverMiddleware(panicHandler)

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 after panic, got %d", w.Code)
	}
}

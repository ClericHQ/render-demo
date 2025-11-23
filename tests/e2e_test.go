package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shahram/prompt-registry/backend/handlers"
	"github.com/shahram/prompt-registry/backend/store"
)

const testPort = "18080"

func TestE2E_CompleteUserFlow(t *testing.T) {
	// Setup test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Initialize store
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// Initialize handlers
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	h := handlers.New(s, logger)

	// Start test server
	server := &http.Server{
		Addr:    ":" + testPort,
		Handler: h.Routes(),
	}

	// Start server in goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Ensure server is shut down after test
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	baseURL := "http://localhost:" + testPort

	// Test 1: Create a prompt
	t.Run("CreatePrompt", func(t *testing.T) {
		payload := map[string]string{
			"slug":        "test-prompt",
			"title":       "Test Prompt",
			"description": "Test Description",
			"content":     "This is version 1",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(baseURL+"/api/prompts", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Failed to create prompt: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify response structure
		if result["slug"] != "test-prompt" {
			t.Errorf("Expected slug 'test-prompt', got %v", result["slug"])
		}

		currentVersion, ok := result["current_version"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected current_version to be an object")
		}

		if currentVersion["version_number"] != float64(1) {
			t.Errorf("Expected version_number 1, got %v", currentVersion["version_number"])
		}

		if currentVersion["content"] != "This is version 1" {
			t.Errorf("Expected content 'This is version 1', got %v", currentVersion["content"])
		}
	})

	// Test 2: List prompts
	t.Run("ListPrompts", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/prompts")
		if err != nil {
			t.Fatalf("Failed to list prompts: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var prompts []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&prompts); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(prompts) != 1 {
			t.Errorf("Expected 1 prompt, got %d", len(prompts))
		}

		if prompts[0]["slug"] != "test-prompt" {
			t.Errorf("Expected slug 'test-prompt', got %v", prompts[0]["slug"])
		}

		if prompts[0]["title"] != "Test Prompt" {
			t.Errorf("Expected title 'Test Prompt', got %v", prompts[0]["title"])
		}
	})

	// Test 3: Get prompt by slug
	t.Run("GetPromptBySlug", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/prompts/test-prompt")
		if err != nil {
			t.Fatalf("Failed to get prompt: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result["slug"] != "test-prompt" {
			t.Errorf("Expected slug 'test-prompt', got %v", result["slug"])
		}

		currentVersion, ok := result["current_version"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected current_version to be an object")
		}

		if currentVersion["content"] != "This is version 1" {
			t.Errorf("Expected content 'This is version 1', got %v", currentVersion["content"])
		}
	})

	// Test 4: Create new version
	t.Run("CreateNewVersion", func(t *testing.T) {
		payload := map[string]string{
			"content": "This is version 2",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(baseURL+"/api/prompts/test-prompt/versions", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Failed to create version: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		currentVersion, ok := result["current_version"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected current_version to be an object")
		}

		if currentVersion["version_number"] != float64(2) {
			t.Errorf("Expected version_number 2, got %v", currentVersion["version_number"])
		}

		if currentVersion["content"] != "This is version 2" {
			t.Errorf("Expected content 'This is version 2', got %v", currentVersion["content"])
		}
	})

	// Test 5: List versions
	t.Run("ListVersions", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/prompts/test-prompt/versions")
		if err != nil {
			t.Fatalf("Failed to list versions: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var versions []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(versions) != 2 {
			t.Errorf("Expected 2 versions, got %d", len(versions))
		}

		// Verify ordering (should be ASC by version_number)
		if versions[0]["version_number"] != float64(1) {
			t.Errorf("Expected first version to be 1, got %v", versions[0]["version_number"])
		}

		if versions[1]["version_number"] != float64(2) {
			t.Errorf("Expected second version to be 2, got %v", versions[1]["version_number"])
		}

		// Verify immutability - version 1 should still have original content
		if versions[0]["content"] != "This is version 1" {
			t.Errorf("Version 1 content changed! Expected 'This is version 1', got %v", versions[0]["content"])
		}

		if versions[1]["content"] != "This is version 2" {
			t.Errorf("Expected version 2 content 'This is version 2', got %v", versions[1]["content"])
		}
	})

	// Test 6: Get specific version
	t.Run("GetSpecificVersions", func(t *testing.T) {
		// Get version 1
		resp1, err := http.Get(baseURL + "/api/prompts/test-prompt/versions/1")
		if err != nil {
			t.Fatalf("Failed to get version 1: %v", err)
		}
		defer resp1.Body.Close()

		if resp1.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp1.StatusCode)
		}

		var v1 map[string]interface{}
		if err := json.NewDecoder(resp1.Body).Decode(&v1); err != nil {
			t.Fatalf("Failed to decode version 1: %v", err)
		}

		if v1["version_number"] != float64(1) {
			t.Errorf("Expected version_number 1, got %v", v1["version_number"])
		}

		if v1["content"] != "This is version 1" {
			t.Errorf("Expected content 'This is version 1', got %v", v1["content"])
		}

		// Get version 2
		resp2, err := http.Get(baseURL + "/api/prompts/test-prompt/versions/2")
		if err != nil {
			t.Fatalf("Failed to get version 2: %v", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp2.StatusCode)
		}

		var v2 map[string]interface{}
		if err := json.NewDecoder(resp2.Body).Decode(&v2); err != nil {
			t.Fatalf("Failed to decode version 2: %v", err)
		}

		if v2["version_number"] != float64(2) {
			t.Errorf("Expected version_number 2, got %v", v2["version_number"])
		}

		if v2["content"] != "This is version 2" {
			t.Errorf("Expected content 'This is version 2', got %v", v2["content"])
		}
	})

	// Test 7: Health check
	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/health")
		if err != nil {
			t.Fatalf("Failed to check health: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var health map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			t.Fatalf("Failed to decode health response: %v", err)
		}

		if health["status"] != "healthy" {
			t.Errorf("Expected status 'healthy', got %v", health["status"])
		}

		if health["database"] != "connected" {
			t.Errorf("Expected database 'connected', got %v", health["database"])
		}
	})

	// Test 8: Metrics
	t.Run("Metrics", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/metrics")
		if err != nil {
			t.Fatalf("Failed to get metrics: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body := make([]byte, resp.ContentLength)
		resp.Body.Read(body)
		metricsText := string(body)

		// Verify metrics exist
		expectedMetrics := []string{
			"prompts_created_total 1",
			"prompt_versions_created_total 2",
			"http_requests_total",
		}

		for _, metric := range expectedMetrics {
			if !containsString(metricsText, metric) {
				t.Errorf("Expected metrics to contain %q", metric)
			}
		}
	})

	t.Log("All E2E tests passed!")
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test pagination
func TestE2E_Pagination(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	h := handlers.New(s, logger)

	server := &http.Server{
		Addr:    ":18081",
		Handler: h.Routes(),
	}

	go func() {
		server.ListenAndServe()
	}()
	time.Sleep(100 * time.Millisecond)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	baseURL := "http://localhost:18081"

	// Create 5 prompts
	for i := 1; i <= 5; i++ {
		payload := map[string]string{
			"title":   fmt.Sprintf("Prompt %d", i),
			"content": fmt.Sprintf("Content %d", i),
		}
		body, _ := json.Marshal(payload)
		http.Post(baseURL+"/api/prompts", "application/json", bytes.NewReader(body))
	}

	// Test pagination
	resp, err := http.Get(baseURL + "/api/prompts?limit=2&offset=0")
	if err != nil {
		t.Fatalf("Failed to get prompts: %v", err)
	}
	defer resp.Body.Close()

	var prompts []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&prompts)

	if len(prompts) != 2 {
		t.Errorf("Expected 2 prompts with limit=2, got %d", len(prompts))
	}

	// Get next page
	resp2, err := http.Get(baseURL + "/api/prompts?limit=2&offset=2")
	if err != nil {
		t.Fatalf("Failed to get prompts: %v", err)
	}
	defer resp2.Body.Close()

	var prompts2 []map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&prompts2)

	if len(prompts2) != 2 {
		t.Errorf("Expected 2 prompts with limit=2&offset=2, got %d", len(prompts2))
	}

	// Verify they're different
	if prompts[0]["slug"] == prompts2[0]["slug"] {
		t.Error("Expected different prompts on different pages")
	}
}

// Test frontend serving and structure
func TestE2E_FrontendServing(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	h := handlers.New(s, logger)

	server := &http.Server{
		Addr:    ":18082",
		Handler: h.Routes(),
	}

	go func() {
		server.ListenAndServe()
	}()
	time.Sleep(100 * time.Millisecond)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	baseURL := "http://localhost:18082"

	// Test that root serves HTML
	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("Failed to get root: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for root, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !containsSubstring(contentType, "text/html") && !containsSubstring(contentType, "text/plain") {
		t.Errorf("Expected HTML content type, got %s", contentType)
	}

	// Read HTML content
	body := make([]byte, 10000)
	n, _ := resp.Body.Read(body)
	html := string(body[:n])

	// Verify it's HTML
	if !containsSubstring(html, "<!DOCTYPE html>") && !containsSubstring(html, "<html") {
		t.Error("Response doesn't appear to be HTML")
	}

	// Verify key elements for Option B design exist
	expectedElements := []string{
		"Prompt Registry",
		"view-list",
		"view-create",
		"view-detail",
	}

	for _, element := range expectedElements {
		if !containsSubstring(html, element) {
			t.Errorf("Expected HTML to contain %q", element)
		}
	}
}

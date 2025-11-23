package store

import (
	"testing"

	"github.com/shahram/prompt-registry/backend/models"
)

func setupTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// Test CreatePrompt
func TestCreatePrompt_Success(t *testing.T) {
	s := setupTestStore(t)

	input := models.CreatePromptInput{
		Title:       "Test Prompt",
		Description: "Test Description",
		Content:     "Test Content",
	}

	result, err := s.CreatePrompt(input)
	if err != nil {
		t.Fatalf("CreatePrompt failed: %v", err)
	}

	if result.Title != input.Title {
		t.Errorf("Expected title %q, got %q", input.Title, result.Title)
	}
	if result.Description != input.Description {
		t.Errorf("Expected description %q, got %q", input.Description, result.Description)
	}
	if result.CurrentVersion.VersionNumber != 1 {
		t.Errorf("Expected version number 1, got %d", result.CurrentVersion.VersionNumber)
	}
	if result.CurrentVersion.Content != input.Content {
		t.Errorf("Expected content %q, got %q", input.Content, result.CurrentVersion.Content)
	}
	if result.Slug == "" {
		t.Error("Expected slug to be auto-generated")
	}
}

func TestCreatePrompt_WithCustomSlug(t *testing.T) {
	s := setupTestStore(t)

	input := models.CreatePromptInput{
		Slug:        "custom-slug",
		Title:       "Test Prompt",
		Description: "Test Description",
		Content:     "Test Content",
	}

	result, err := s.CreatePrompt(input)
	if err != nil {
		t.Fatalf("CreatePrompt failed: %v", err)
	}

	if result.Slug != "custom-slug" {
		t.Errorf("Expected slug %q, got %q", "custom-slug", result.Slug)
	}
}

func TestCreatePrompt_AutoGenerateSlug(t *testing.T) {
	s := setupTestStore(t)

	input := models.CreatePromptInput{
		Title:   "My Test Prompt",
		Content: "Test Content",
	}

	result, err := s.CreatePrompt(input)
	if err != nil {
		t.Fatalf("CreatePrompt failed: %v", err)
	}

	// Should generate slug from title
	if result.Slug != "my-test-prompt" {
		t.Errorf("Expected slug %q, got %q", "my-test-prompt", result.Slug)
	}
}

func TestCreatePrompt_EmptyTitle(t *testing.T) {
	s := setupTestStore(t)

	input := models.CreatePromptInput{
		Title:   "",
		Content: "Test Content",
	}

	_, err := s.CreatePrompt(input)
	if err == nil {
		t.Error("Expected error for empty title, got nil")
	}
}

func TestCreatePrompt_EmptyContent(t *testing.T) {
	s := setupTestStore(t)

	input := models.CreatePromptInput{
		Title:   "Test Title",
		Content: "",
	}

	_, err := s.CreatePrompt(input)
	if err == nil {
		t.Error("Expected error for empty content, got nil")
	}
}

func TestCreatePrompt_DuplicateSlug(t *testing.T) {
	s := setupTestStore(t)

	input := models.CreatePromptInput{
		Slug:    "duplicate-slug",
		Title:   "Test Prompt 1",
		Content: "Test Content 1",
	}

	_, err := s.CreatePrompt(input)
	if err != nil {
		t.Fatalf("First CreatePrompt failed: %v", err)
	}

	// Try to create another with same slug
	input2 := models.CreatePromptInput{
		Slug:    "duplicate-slug",
		Title:   "Test Prompt 2",
		Content: "Test Content 2",
	}

	_, err = s.CreatePrompt(input2)
	if err == nil {
		t.Error("Expected error for duplicate slug, got nil")
	}
}

// Test CreatePromptVersion
func TestCreatePromptVersion_Success(t *testing.T) {
	s := setupTestStore(t)

	// Create initial prompt
	input := models.CreatePromptInput{
		Slug:    "test-prompt",
		Title:   "Test Prompt",
		Content: "Version 1 Content",
	}
	_, err := s.CreatePrompt(input)
	if err != nil {
		t.Fatalf("CreatePrompt failed: %v", err)
	}

	// Create new version
	versionInput := models.CreatePromptVersionInput{
		Content: "Version 2 Content",
	}
	result, err := s.CreatePromptVersion("test-prompt", versionInput)
	if err != nil {
		t.Fatalf("CreatePromptVersion failed: %v", err)
	}

	if result.CurrentVersion.VersionNumber != 2 {
		t.Errorf("Expected version number 2, got %d", result.CurrentVersion.VersionNumber)
	}
	if result.CurrentVersion.Content != "Version 2 Content" {
		t.Errorf("Expected content %q, got %q", "Version 2 Content", result.CurrentVersion.Content)
	}
}

func TestCreatePromptVersion_NonExistentSlug(t *testing.T) {
	s := setupTestStore(t)

	versionInput := models.CreatePromptVersionInput{
		Content: "Test Content",
	}

	_, err := s.CreatePromptVersion("non-existent", versionInput)
	if err == nil {
		t.Error("Expected error for non-existent slug, got nil")
	}
}

func TestCreatePromptVersion_EmptyContent(t *testing.T) {
	s := setupTestStore(t)

	// Create initial prompt
	input := models.CreatePromptInput{
		Slug:    "test-prompt",
		Title:   "Test Prompt",
		Content: "Version 1 Content",
	}
	_, err := s.CreatePrompt(input)
	if err != nil {
		t.Fatalf("CreatePrompt failed: %v", err)
	}

	// Try to create version with empty content
	versionInput := models.CreatePromptVersionInput{
		Content: "",
	}

	_, err = s.CreatePromptVersion("test-prompt", versionInput)
	if err == nil {
		t.Error("Expected error for empty content, got nil")
	}
}

func TestCreatePromptVersion_Immutability(t *testing.T) {
	s := setupTestStore(t)

	// Create initial prompt
	input := models.CreatePromptInput{
		Slug:    "test-prompt",
		Title:   "Test Prompt",
		Content: "Version 1 Content",
	}
	_, err := s.CreatePrompt(input)
	if err != nil {
		t.Fatalf("CreatePrompt failed: %v", err)
	}

	// Create new version
	versionInput := models.CreatePromptVersionInput{
		Content: "Version 2 Content",
	}
	_, err = s.CreatePromptVersion("test-prompt", versionInput)
	if err != nil {
		t.Fatalf("CreatePromptVersion failed: %v", err)
	}

	// Get version 1 and verify it hasn't changed
	v1, err := s.GetPromptVersion("test-prompt", 1)
	if err != nil {
		t.Fatalf("GetPromptVersion failed: %v", err)
	}

	if v1.Content != "Version 1 Content" {
		t.Errorf("Version 1 content was modified! Expected %q, got %q", "Version 1 Content", v1.Content)
	}
}

// Test GetPromptBySlug
func TestGetPromptBySlug_Success(t *testing.T) {
	s := setupTestStore(t)

	input := models.CreatePromptInput{
		Slug:    "test-prompt",
		Title:   "Test Prompt",
		Content: "Test Content",
	}
	_, err := s.CreatePrompt(input)
	if err != nil {
		t.Fatalf("CreatePrompt failed: %v", err)
	}

	result, err := s.GetPromptBySlug("test-prompt")
	if err != nil {
		t.Fatalf("GetPromptBySlug failed: %v", err)
	}

	if result.Slug != "test-prompt" {
		t.Errorf("Expected slug %q, got %q", "test-prompt", result.Slug)
	}
	if result.Title != "Test Prompt" {
		t.Errorf("Expected title %q, got %q", "Test Prompt", result.Title)
	}
	if result.CurrentVersion.Content != "Test Content" {
		t.Errorf("Expected content %q, got %q", "Test Content", result.CurrentVersion.Content)
	}
}

func TestGetPromptBySlug_NotFound(t *testing.T) {
	s := setupTestStore(t)

	_, err := s.GetPromptBySlug("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent slug, got nil")
	}
}

// Test GetPromptVersion
func TestGetPromptVersion_Success(t *testing.T) {
	s := setupTestStore(t)

	// Create prompt with initial version
	input := models.CreatePromptInput{
		Slug:    "test-prompt",
		Title:   "Test Prompt",
		Content: "Version 1 Content",
	}
	_, err := s.CreatePrompt(input)
	if err != nil {
		t.Fatalf("CreatePrompt failed: %v", err)
	}

	// Get version 1
	v1, err := s.GetPromptVersion("test-prompt", 1)
	if err != nil {
		t.Fatalf("GetPromptVersion failed: %v", err)
	}

	if v1.VersionNumber != 1 {
		t.Errorf("Expected version number 1, got %d", v1.VersionNumber)
	}
	if v1.Content != "Version 1 Content" {
		t.Errorf("Expected content %q, got %q", "Version 1 Content", v1.Content)
	}
}

func TestGetPromptVersion_NonExistentSlug(t *testing.T) {
	s := setupTestStore(t)

	_, err := s.GetPromptVersion("non-existent", 1)
	if err == nil {
		t.Error("Expected error for non-existent slug, got nil")
	}
}

func TestGetPromptVersion_NonExistentVersion(t *testing.T) {
	s := setupTestStore(t)

	// Create prompt with only version 1
	input := models.CreatePromptInput{
		Slug:    "test-prompt",
		Title:   "Test Prompt",
		Content: "Version 1 Content",
	}
	_, err := s.CreatePrompt(input)
	if err != nil {
		t.Fatalf("CreatePrompt failed: %v", err)
	}

	// Try to get version 2 (doesn't exist)
	_, err = s.GetPromptVersion("test-prompt", 2)
	if err == nil {
		t.Error("Expected error for non-existent version, got nil")
	}
}

// Test ListPrompts
func TestListPrompts_Success(t *testing.T) {
	s := setupTestStore(t)

	// Create multiple prompts
	for i := 1; i <= 3; i++ {
		input := models.CreatePromptInput{
			Title:   "Test Prompt " + string(rune('0'+i)),
			Content: "Test Content",
		}
		_, err := s.CreatePrompt(input)
		if err != nil {
			t.Fatalf("CreatePrompt failed: %v", err)
		}
	}

	// List all prompts
	results, err := s.ListPrompts(10, 0)
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 prompts, got %d", len(results))
	}

	// Verify all created prompts are present
	titles := make(map[string]bool)
	for _, p := range results {
		titles[p.Title] = true
	}
	for i := 1; i <= 3; i++ {
		expectedTitle := "Test Prompt " + string(rune('0'+i))
		if !titles[expectedTitle] {
			t.Errorf("Expected to find prompt %q", expectedTitle)
		}
	}
}

func TestListPrompts_LimitAndOffset(t *testing.T) {
	s := setupTestStore(t)

	// Create 5 prompts
	for i := 1; i <= 5; i++ {
		input := models.CreatePromptInput{
			Title:   "Prompt " + string(rune('0'+i)),
			Content: "Test Content",
		}
		_, err := s.CreatePrompt(input)
		if err != nil {
			t.Fatalf("CreatePrompt failed: %v", err)
		}
	}

	// Get first 2
	results, err := s.ListPrompts(2, 0)
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 prompts, got %d", len(results))
	}

	// Get next 2 (with offset)
	results2, err := s.ListPrompts(2, 2)
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}
	if len(results2) != 2 {
		t.Errorf("Expected 2 prompts, got %d", len(results2))
	}

	// Verify they're different
	if results[0].Slug == results2[0].Slug {
		t.Error("Expected different prompts with offset")
	}
}

func TestListPrompts_Empty(t *testing.T) {
	s := setupTestStore(t)

	results, err := s.ListPrompts(10, 0)
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 prompts, got %d", len(results))
	}
}

// Test ListPromptVersions
func TestListPromptVersions_Success(t *testing.T) {
	s := setupTestStore(t)

	// Create prompt
	input := models.CreatePromptInput{
		Slug:    "test-prompt",
		Title:   "Test Prompt",
		Content: "Version 1",
	}
	_, err := s.CreatePrompt(input)
	if err != nil {
		t.Fatalf("CreatePrompt failed: %v", err)
	}

	// Create additional versions
	for i := 2; i <= 3; i++ {
		versionInput := models.CreatePromptVersionInput{
			Content: "Version " + string(rune('0'+i)),
		}
		_, err := s.CreatePromptVersion("test-prompt", versionInput)
		if err != nil {
			t.Fatalf("CreatePromptVersion failed: %v", err)
		}
	}

	// List versions
	versions, err := s.ListPromptVersions("test-prompt")
	if err != nil {
		t.Fatalf("ListPromptVersions failed: %v", err)
	}

	if len(versions) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(versions))
	}

	// Verify ordering (ASC by version_number)
	if versions[0].VersionNumber != 1 {
		t.Errorf("Expected first version to be 1, got %d", versions[0].VersionNumber)
	}
	if versions[2].VersionNumber != 3 {
		t.Errorf("Expected last version to be 3, got %d", versions[2].VersionNumber)
	}
}

func TestListPromptVersions_NonExistentSlug(t *testing.T) {
	s := setupTestStore(t)

	_, err := s.ListPromptVersions("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent slug, got nil")
	}
}

// Test GetStats
func TestGetStats_Success(t *testing.T) {
	s := setupTestStore(t)

	// Initially should be 0
	stats, err := s.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalPrompts != 0 {
		t.Errorf("Expected 0 prompts, got %d", stats.TotalPrompts)
	}
	if stats.TotalPromptVersions != 0 {
		t.Errorf("Expected 0 versions, got %d", stats.TotalPromptVersions)
	}

	// Create 2 prompts
	for i := 1; i <= 2; i++ {
		input := models.CreatePromptInput{
			Title:   "Prompt " + string(rune('0'+i)),
			Content: "Content",
		}
		_, err := s.CreatePrompt(input)
		if err != nil {
			t.Fatalf("CreatePrompt failed: %v", err)
		}
	}

	// Check stats (2 prompts, 2 versions - one per prompt)
	stats, err = s.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalPrompts != 2 {
		t.Errorf("Expected 2 prompts, got %d", stats.TotalPrompts)
	}
	if stats.TotalPromptVersions != 2 {
		t.Errorf("Expected 2 versions, got %d", stats.TotalPromptVersions)
	}

	// Create another version for first prompt
	versionInput := models.CreatePromptVersionInput{
		Content: "New Version",
	}
	_, err = s.CreatePromptVersion("prompt-1", versionInput)
	if err != nil {
		t.Fatalf("CreatePromptVersion failed: %v", err)
	}

	// Check stats (still 2 prompts, now 3 versions)
	stats, err = s.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalPrompts != 2 {
		t.Errorf("Expected 2 prompts, got %d", stats.TotalPrompts)
	}
	if stats.TotalPromptVersions != 3 {
		t.Errorf("Expected 3 versions, got %d", stats.TotalPromptVersions)
	}
}

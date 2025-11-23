package models

import "time"

// Prompt represents a logical prompt container
type Prompt struct {
	ID             int64     `json:"id"`
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	CurrentVersion int       `json:"current_version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// PromptVersion represents an immutable version of a prompt
type PromptVersion struct {
	ID            int64     `json:"id"`
	PromptID      int64     `json:"prompt_id"`
	VersionNumber int       `json:"version_number"`
	Content       string    `json:"content"`
	CreatedAt     time.Time `json:"created_at"`
}

// PromptSummary represents a prompt in list view
type PromptSummary struct {
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	CurrentVersion int       `json:"current_version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// PromptWithCurrentVersion represents a prompt with its current version
type PromptWithCurrentVersion struct {
	Slug           string        `json:"slug"`
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	CurrentVersion PromptVersion `json:"current_version"`
}

// Stats represents system-wide statistics
type Stats struct {
	TotalPrompts        int `json:"total_prompts"`
	TotalPromptVersions int `json:"total_prompt_versions"`
}

// CreatePromptInput represents input for creating a new prompt
type CreatePromptInput struct {
	Slug        string `json:"slug"`        // optional, auto-generated from title if empty
	Title       string `json:"title"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// CreatePromptVersionInput represents input for creating a new version
type CreatePromptVersionInput struct {
	Content string `json:"content"`
}

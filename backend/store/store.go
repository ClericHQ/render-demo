package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/shahram/prompt-registry/backend/models"
)

// Store defines the interface for prompt storage operations
type Store interface {
	CreatePrompt(input models.CreatePromptInput) (models.PromptWithCurrentVersion, error)
	CreatePromptVersion(slug string, input models.CreatePromptVersionInput) (models.PromptWithCurrentVersion, error)
	GetPromptBySlug(slug string) (models.PromptWithCurrentVersion, error)
	GetPromptVersion(slug string, version int) (models.PromptVersion, error)
	ListPrompts(limit, offset int) ([]models.PromptSummary, error)
	ListPromptVersions(slug string) ([]models.PromptVersion, error)
	GetStats() (models.Stats, error)
	Close() error
}

// SQLiteStore implements the Store interface using SQLite
type SQLiteStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// New creates a new SQLiteStore and initializes the database
func New(dbPath string) (*SQLiteStore, error) {
	logger := slog.Default()

	// Remove sqlite3:// prefix if present
	cleanPath := strings.TrimPrefix(dbPath, "sqlite3://")
	db, err := sql.Open("sqlite3", cleanPath)
	if err != nil {
		logger.Error("failed to open database", "error", err, "path", dbPath)
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SQLiteStore{
		db:     db,
		logger: logger,
	}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	logger.Info("database initialized", "path", dbPath)
	return store, nil
}

// initSchema creates the database tables if they don't exist
func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS prompts (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		slug             TEXT UNIQUE NOT NULL,
		title            TEXT NOT NULL,
		description      TEXT,
		current_version  INTEGER NOT NULL DEFAULT 0,
		created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS prompt_versions (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		prompt_id      INTEGER NOT NULL,
		version_number INTEGER NOT NULL,
		content        TEXT NOT NULL,
		created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(prompt_id) REFERENCES prompts(id),
		UNIQUE(prompt_id, version_number)
	);
	`

	if _, err := s.db.Exec(schema); err != nil {
		s.logger.Error("failed to initialize schema", "error", err)
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}

// generateSlug creates a URL-friendly slug from a title
func generateSlug(title string) string {
	// Convert to lowercase
	slug := strings.ToLower(title)
	// Replace spaces with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove any character that's not alphanumeric or hyphen
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// CreatePrompt creates a new prompt with an initial version
func (s *SQLiteStore) CreatePrompt(input models.CreatePromptInput) (models.PromptWithCurrentVersion, error) {
	start := time.Now()
	var result models.PromptWithCurrentVersion

	// Validate input
	if strings.TrimSpace(input.Title) == "" {
		return result, errors.New("title cannot be empty")
	}
	if strings.TrimSpace(input.Content) == "" {
		return result, errors.New("content cannot be empty")
	}

	// Generate slug if not provided
	slug := input.Slug
	if slug == "" {
		slug = generateSlug(input.Title)
	}

	// Begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		s.logger.Error("failed to begin transaction", "error", err)
		return result, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert prompt
	promptResult, err := tx.Exec(
		`INSERT INTO prompts (slug, title, description, current_version) VALUES (?, ?, ?, 0)`,
		slug, input.Title, input.Description,
	)
	if err != nil {
		s.logger.Error("failed to insert prompt", "error", err, "slug", slug)
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return result, fmt.Errorf("prompt with slug %q already exists", slug)
		}
		return result, fmt.Errorf("failed to insert prompt: %w", err)
	}

	promptID, err := promptResult.LastInsertId()
	if err != nil {
		s.logger.Error("failed to get prompt ID", "error", err)
		return result, fmt.Errorf("failed to get prompt ID: %w", err)
	}

	// Insert initial version
	versionResult, err := tx.Exec(
		`INSERT INTO prompt_versions (prompt_id, version_number, content) VALUES (?, 0, ?)`,
		promptID, input.Content,
	)
	if err != nil {
		s.logger.Error("failed to insert version", "error", err, "prompt_id", promptID)
		return result, fmt.Errorf("failed to insert version: %w", err)
	}

	versionID, err := versionResult.LastInsertId()
	if err != nil {
		s.logger.Error("failed to get version ID", "error", err)
		return result, fmt.Errorf("failed to get version ID: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction", "error", err)
		return result, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Build result
	result = models.PromptWithCurrentVersion{
		Slug:        slug,
		Title:       input.Title,
		Description: input.Description,
		CurrentVersion: models.PromptVersion{
			ID:            versionID,
			PromptID:      promptID,
			VersionNumber: 1,
			Content:       input.Content,
		},
	}

	duration := time.Since(start)
	s.logger.Info("database operation",
		"operation", "CreatePrompt",
		"slug", slug,
		"prompt_id", promptID,
		"duration_ms", duration.Milliseconds(),
	)
	return result, nil
}

// CreatePromptVersion creates a new version for an existing prompt
func (s *SQLiteStore) CreatePromptVersion(slug string, input models.CreatePromptVersionInput) (models.PromptWithCurrentVersion, error) {
	start := time.Now()
	var result models.PromptWithCurrentVersion

	// Validate input
	if strings.TrimSpace(input.Content) == "" {
		return result, errors.New("content cannot be empty")
	}

	// Begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		s.logger.Error("failed to begin transaction", "error", err)
		return result, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get prompt
	var promptID int64
	var title, description string
	var currentVersion int
	err = tx.QueryRow(
		`SELECT id, title, description, current_version FROM prompts WHERE slug = ?`,
		slug,
	).Scan(&promptID, &title, &description, &currentVersion)
	if err == sql.ErrNoRows {
		return result, fmt.Errorf("prompt with slug %q not found", slug)
	}
	if err != nil {
		s.logger.Error("failed to get prompt", "error", err, "slug", slug)
		return result, fmt.Errorf("failed to get prompt: %w", err)
	}

	// Calculate new version number
	newVersionNumber := currentVersion + 1

	// Insert new version
	versionResult, err := tx.Exec(
		`INSERT INTO prompt_versions (prompt_id, version_number, content) VALUES (?, ?, ?)`,
		promptID, newVersionNumber, input.Content,
	)
	if err != nil {
		s.logger.Error("failed to insert version", "error", err, "prompt_id", promptID)
		return result, fmt.Errorf("failed to insert version: %w", err)
	}

	versionID, err := versionResult.LastInsertId()
	if err != nil {
		s.logger.Error("failed to get version ID", "error", err)
		return result, fmt.Errorf("failed to get version ID: %w", err)
	}

	// Update prompt's current_version and updated_at
	_, err = tx.Exec(
		`UPDATE prompts SET current_version = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		newVersionNumber, promptID,
	)
	if err != nil {
		s.logger.Error("failed to update prompt", "error", err, "prompt_id", promptID)
		return result, fmt.Errorf("failed to update prompt: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction", "error", err)
		return result, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Build result
	result = models.PromptWithCurrentVersion{
		Slug:        slug,
		Title:       title,
		Description: description,
		CurrentVersion: models.PromptVersion{
			ID:            versionID,
			PromptID:      promptID,
			VersionNumber: newVersionNumber,
			Content:       input.Content,
		},
	}

	duration := time.Since(start)
	s.logger.Info("database operation",
		"operation", "CreatePromptVersion",
		"slug", slug,
		"version", newVersionNumber,
		"duration_ms", duration.Milliseconds(),
	)
	return result, nil
}

// GetPromptBySlug retrieves a prompt with its current version
func (s *SQLiteStore) GetPromptBySlug(slug string) (models.PromptWithCurrentVersion, error) {
	start := time.Now()
	var result models.PromptWithCurrentVersion

	// Get prompt with current version in a single query
	err := s.db.QueryRow(`
		SELECT
			p.slug, p.title, p.description,
			pv.id, pv.prompt_id, pv.version_number, pv.content, pv.created_at
		FROM prompts p
		JOIN prompt_versions pv ON p.id = pv.prompt_id AND pv.version_number = p.current_version
		WHERE p.slug = ?
	`, slug).Scan(
		&result.Slug, &result.Title, &result.Description,
		&result.CurrentVersion.ID, &result.CurrentVersion.PromptID,
		&result.CurrentVersion.VersionNumber, &result.CurrentVersion.Content,
		&result.CurrentVersion.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return result, fmt.Errorf("prompt with slug %q not found", slug)
	}
	if err != nil {
		s.logger.Error("failed to get prompt", "error", err, "slug", slug)
		return result, fmt.Errorf("failed to get prompt: %w", err)
	}

	duration := time.Since(start)
	s.logger.Info("database operation",
		"operation", "GetPromptBySlug",
		"slug", slug,
		"duration_ms", duration.Milliseconds(),
	)
	return result, nil
}

// GetPromptVersion retrieves a specific version of a prompt
func (s *SQLiteStore) GetPromptVersion(slug string, version int) (models.PromptVersion, error) {
	start := time.Now()
	var result models.PromptVersion

	err := s.db.QueryRow(`
		SELECT pv.id, pv.prompt_id, pv.version_number, pv.content, pv.created_at
		FROM prompt_versions pv
		JOIN prompts p ON p.id = pv.prompt_id
		WHERE p.slug = ? AND pv.version_number = ?
	`, slug, version).Scan(
		&result.ID, &result.PromptID, &result.VersionNumber,
		&result.Content, &result.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return result, fmt.Errorf("version %d not found for prompt %q", version, slug)
	}
	if err != nil {
		s.logger.Error("failed to get version", "error", err, "slug", slug, "version", version)
		return result, fmt.Errorf("failed to get version: %w", err)
	}

	duration := time.Since(start)
	s.logger.Info("database operation",
		"operation", "GetPromptVersion",
		"slug", slug,
		"version", version,
		"duration_ms", duration.Milliseconds(),
	)
	return result, nil
}

// ListPrompts retrieves prompts ordered by created_at DESC
func (s *SQLiteStore) ListPrompts(limit, offset int) ([]models.PromptSummary, error) {
	start := time.Now()
	rows, err := s.db.Query(`
		SELECT slug, title, description, current_version, created_at, updated_at
		FROM prompts
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		s.logger.Error("failed to list prompts", "error", err)
		return nil, fmt.Errorf("failed to list prompts: %w", err)
	}
	defer rows.Close()

	var results []models.PromptSummary
	for rows.Next() {
		var summary models.PromptSummary
		err := rows.Scan(
			&summary.Slug, &summary.Title, &summary.Description,
			&summary.CurrentVersion, &summary.CreatedAt, &summary.UpdatedAt,
		)
		if err != nil {
			s.logger.Error("failed to scan prompt", "error", err)
			return nil, fmt.Errorf("failed to scan prompt: %w", err)
		}
		results = append(results, summary)
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("failed to iterate prompts", "error", err)
		return nil, fmt.Errorf("failed to iterate prompts: %w", err)
	}

	// Return empty slice instead of nil
	if results == nil {
		results = []models.PromptSummary{}
	}

	duration := time.Since(start)
	s.logger.Info("database operation",
		"operation", "ListPrompts",
		"limit", limit,
		"offset", offset,
		"rows_returned", len(results),
		"duration_ms", duration.Milliseconds(),
	)
	return results, nil
}

// ListPromptVersions retrieves all versions for a prompt
func (s *SQLiteStore) ListPromptVersions(slug string) ([]models.PromptVersion, error) {
	start := time.Now()
	// First verify the prompt exists
	var promptID int64
	err := s.db.QueryRow(`SELECT id FROM prompts WHERE slug = ?`, slug).Scan(&promptID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("prompt with slug %q not found", slug)
	}
	if err != nil {
		s.logger.Error("failed to get prompt", "error", err, "slug", slug)
		return nil, fmt.Errorf("failed to get prompt: %w", err)
	}

	// Get all versions
	rows, err := s.db.Query(`
		SELECT id, prompt_id, version_number, content, created_at
		FROM prompt_versions
		WHERE prompt_id = ?
		ORDER BY version_number ASC
	`, promptID)
	if err != nil {
		s.logger.Error("failed to list versions", "error", err, "slug", slug)
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}
	defer rows.Close()

	var results []models.PromptVersion
	for rows.Next() {
		var version models.PromptVersion
		err := rows.Scan(
			&version.ID, &version.PromptID, &version.VersionNumber,
			&version.Content, &version.CreatedAt,
		)
		if err != nil {
			s.logger.Error("failed to scan version", "error", err)
			return nil, fmt.Errorf("failed to scan version: %w", err)
		}
		results = append(results, version)
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("failed to iterate versions", "error", err)
		return nil, fmt.Errorf("failed to iterate versions: %w", err)
	}

	duration := time.Since(start)
	s.logger.Info("database operation",
		"operation", "ListPromptVersions",
		"slug", slug,
		"rows_returned", len(results),
		"duration_ms", duration.Milliseconds(),
	)
	return results, nil
}

// GetStats retrieves system-wide statistics
func (s *SQLiteStore) GetStats() (models.Stats, error) {
	start := time.Now()
	var stats models.Stats

	// Get total prompts
	err := s.db.QueryRow(`SELECT COUNT(*) FROM prompts`).Scan(&stats.TotalPrompts)
	if err != nil {
		s.logger.Error("failed to count prompts", "error", err)
		return stats, fmt.Errorf("failed to count prompts: %w", err)
	}

	// Get total versions
	err = s.db.QueryRow(`SELECT COUNT(*) FROM prompt_versions`).Scan(&stats.TotalPromptVersions)
	if err != nil {
		s.logger.Error("failed to count versions", "error", err)
		return stats, fmt.Errorf("failed to count versions: %w", err)
	}

	duration := time.Since(start)
	s.logger.Info("database operation",
		"operation", "GetStats",
		"total_prompts", stats.TotalPrompts,
		"total_versions", stats.TotalPromptVersions,
		"duration_ms", duration.Milliseconds(),
	)
	return stats, nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	if err := s.db.Close(); err != nil {
		s.logger.Error("failed to close database", "error", err)
		return fmt.Errorf("failed to close database: %w", err)
	}
	s.logger.Info("database closed")
	return nil
}

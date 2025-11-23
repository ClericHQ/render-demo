# Minimal Go Prompt Registry – TDD Implementation Guide

Create a minimal Go Prompt Registry following Test-Driven Development principles.

- Stores prompts identified by a slug.
- Each prompt has immutable versions (append-only, version_number 1,2,3…).
- Anyone can create prompts and new versions; no auth.
- Public JSON API + minimal Tailwind HTML frontend.

## Project Structure

- `/cmd/server/main.go` – Application entry point
- `/backend/store/store.go` – SQLite database implementation
- `/backend/handlers/handlers.go` – HTTP handlers with middleware
- `/backend/handlers/frontend.html` – Single-page frontend embedded in Go binary
- `/backend/handlers/metrics.go` – Prometheus metrics tracking
- `/backend/models/models.go` – Data types
- `/tests/e2e_test.go` – Integration tests
- `/README.md` – Essential documentation
- `/Makefile` – Simple development commands
- `/go.mod` – Module definition
- `/render.yaml` – Render deployment configuration

Technology Stack: Go 1.25.0, standard library HTTP server, SQLite (`mattn/go-sqlite3`)

## README Requirements

Keep it concise – only essential information:

- Quick Start (how to run locally)
- Project structure overview
- API endpoints specification
- Database schema
- Configuration (environment variables)
- Development commands (Makefile)
- Observability (logs, metrics, health checks)

Do **not** include architecture justifications, scaling advice, deployment guides, or excessive detail.

Create symlink: `ln -s README.md agents.md`

## Step 1: Store Layer (TDD)

Write tests **first** in `backend/store/store_test.go`.

### 1.1 Data Types (for tests and implementation)

Define in `backend/models/models.go`:

**Prompt – logical prompt container**

```go
type Prompt struct {
    ID               int64
    Slug             string
    Title            string
    Description      string
    CurrentVersion   int
    CreatedAt        time.Time
    UpdatedAt        time.Time
}
```

**PromptVersion – immutable version**

```go
type PromptVersion struct {
    ID            int64
    PromptID      int64
    VersionNumber int
    Content       string
    CreatedAt     time.Time
}
```

**PromptSummary – list view**

```go
type PromptSummary struct {
    Slug           string
    Title          string
    Description    string
    CurrentVersion int
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

**PromptWithCurrentVersion – detail view**

```go
type PromptWithCurrentVersion struct {
    Slug            string
    Title           string
    Description     string
    CurrentVersion  PromptVersion
}
```

**Stats**

```go
type Stats struct {
    TotalPrompts        int
    TotalPromptVersions int
}
```

**CreatePromptInput**

```go
type CreatePromptInput struct {
    Slug        string // optional, auto-generated from title if empty
    Title       string
    Description string
    Content     string
}
```

**CreatePromptVersionInput**

```go
type CreatePromptVersionInput struct {
    Content string
}
```

### 1.2 Store Interface (for tests)

**Database Support:**
- SQLite for all environments (`:memory:` for tests, `./data/prompts.db` for local, persistent disk for production)
- Simple, reliable, no external database dependencies

In tests, assume the following interface in `backend/store`:

```go
type Store interface {
    CreatePrompt(input CreatePromptInput) (PromptWithCurrentVersion, error)
    CreatePromptVersion(slug string, input CreatePromptVersionInput) (PromptWithCurrentVersion, error)
    GetPromptBySlug(slug string) (PromptWithCurrentVersion, error)
    GetPromptVersion(slug string, version int) (PromptVersion, error)
    ListPrompts(limit, offset int) ([]PromptSummary, error)
    ListPromptVersions(slug string) ([]PromptVersion, error)
    GetStats() (Stats, error)
    Close() error
}
```

### 1.3 Tests to Write

Use real in-memory SQLite (`:memory:`), no mocking.

Helper:

```go
func setupTestStore(t *testing.T) *SQLiteStore {
    t.Helper()
    s, err := store.New(":memory:")
    if err != nil {
        t.Fatalf("Failed to create test store: %v", err)
    }
    t.Cleanup(func() { s.Close() })
    return s
}
```

Test these methods:

1. **CreatePrompt(input CreatePromptInput) (PromptWithCurrentVersion, error)**
   - Creates a new prompt and an initial version (version_number = 1).
   - Auto-generates a slug from title if `input.Slug` is empty (simple kebab-case).
   - Returns error for empty title, empty content, or duplicate slug.
   - Sets `CurrentVersion.VersionNumber == 1`.
2. **CreatePromptVersion(slug string, input CreatePromptVersionInput) (PromptWithCurrentVersion, error)**
   - Finds prompt by slug.
   - Creates a new `prompt_versions` row with `version_number = previous_max + 1`.
   - Updates prompt’s `CurrentVersion`.
   - Returns error if slug not found or content is empty.
   - Does not modify existing versions (immutability).
3. **GetPromptBySlug(slug string) (PromptWithCurrentVersion, error)**
   - Returns prompt with its current version.
   - Returns error if prompt does not exist.
4. **GetPromptVersion(slug string, version int) (PromptVersion, error)**
   - Returns the specific version for the prompt (by slug + version_number).
   - Returns error if prompt or version doesn’t exist.
5. **ListPrompts(limit, offset int) ([]PromptSummary, error)**
   - Returns prompts ordered by `created_at DESC`.
   - Each summary includes current version number.
   - Respects limit and offset.
6. **ListPromptVersions(slug string) ([]PromptVersion, error)**
   - Returns all versions for a prompt ordered by `version_number ASC`.
   - Returns error if slug not found.
7. **GetStats() (Stats, error)**
   - Returns accurate counts for `TotalPrompts` and `TotalPromptVersions`.

Run: `go test ./backend/store -v`  
Expected: **FAIL** (functions don’t exist yet) ✓

### 1.4 Implement `backend/store/store.go`

SQLite schema:

```sql
CREATE TABLE prompts (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  slug             TEXT UNIQUE NOT NULL,
  title            TEXT NOT NULL,
  description      TEXT,
  current_version  INTEGER NOT NULL DEFAULT 0,
  created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE prompt_versions (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  prompt_id      INTEGER NOT NULL,
  version_number INTEGER NOT NULL,
  content        TEXT NOT NULL,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(prompt_id) REFERENCES prompts(id),
  UNIQUE(prompt_id, version_number)
);
```

Requirements:

- Implement all methods tested above.
- Initialize database and create tables in `New()` constructor (idempotent).
- Use structured logging (`log/slog`) for database operations and errors.
- Keep implementation straightforward; no extra behavior beyond tests.

Run: `go test ./backend/store -v`  
Expected: **All tests PASS** ✓

## Step 2: Handler Layer (TDD)

Write tests **first** in `backend/handlers/handlers_test.go`.

Use real in-memory SQLite (`:memory:`) store and `httptest` package. No `MockStore`.

Assume a `Handler` struct:

```go
type Handler struct {
    Store  store.Store
    Logger *slog.Logger
}
```

### 2.1 API Endpoints to Test

1. **POST /api/prompts**
   - Request JSON:

```json
{
  "slug": "optional-slug",
  "title": "Required title",
  "description": "Optional description",
  "content": "Prompt content"
}
```

   - Valid request: returns `201 Created`; JSON includes slug, title, description, and current version (version_number = 1, content).
   - Empty title or content: returns `400 Bad Request`.
   - Duplicate slug: returns `409 Conflict`.
   - Malformed JSON: returns `400 Bad Request`.

2. **GET /api/prompts**
   - Returns JSON array of prompt summaries.
   - Each element includes: slug, title, description, current_version, created_at, updated_at.
   - Optional query parameters `limit` and `offset` (ints) for pagination.
   - Returns `200 OK`.

3. **GET /api/prompts/{slug}**
   - Returns current version for given slug.
   - Response includes: slug, title, description, and current_version object with version_number, content, created_at.
   - Non-existent slug: returns `404 Not Found`.

4. **GET /api/prompts/{slug}/versions**
   - Returns JSON array of versions for the slug.
   - Each version includes: version_number, content, created_at.
   - Non-existent slug: returns `404 Not Found`.

5. **POST /api/prompts/{slug}/versions**
   - Request JSON:

```json
{
  "content": "New prompt content"
}
```

   - Valid request: creates new version (`version_number` incremented), returns `201 Created` with updated prompt and new current version.
   - Missing or empty content: returns `400 Bad Request`.
   - Non-existent slug: returns `404 Not Found`.

6. **GET /api/prompts/{slug}/versions/{version}**
   - Returns specific version for a prompt.
   - Response includes version_number, content, created_at.
   - Non-existent slug or version: returns `404 Not Found`.

7. **GET /health**
   - Uses `store.GetStats()` to verify database connectivity.
   - Returns `200 OK` with JSON body:

```json
{
  "status": "healthy",
  "database": "connected"
}
```

   - If `GetStats()` errors, return `500` with `"database": "error"`.

8. **GET /metrics**
   - Returns Prometheus text format metrics.
   - At minimum, export counters: `prompts_created_total`, `prompt_versions_created_total`, `http_requests_total`, `http_errors_total`.

### 2.2 Middleware Requirements

In tests, validate behavior via status codes and JSON, not by inspecting logs.

Implement in `handlers.go`:

- Request logging: log method, path, status, duration.
- Panic recovery: recover from panics, log, return 500 instead of crashing.
- CORS headers: `Access-Control-Allow-Origin: *`, `Access-Control-Allow-Headers: Content-Type`; handle `OPTIONS` generically.

Use standard library `http.ServeMux` (no external router).

Run: `go test ./backend/handlers -v`  
Expected: **FAIL** (handlers don’t exist yet) ✓

Now implement `backend/handlers/handlers.go` and `backend/handlers/metrics.go`:

- Create `Handler` struct holding a `store.Store`.
- Wire routes and handlers using `http.ServeMux`.
- Implement all endpoints tested above.
- Implement metrics with `sync/atomic` counters.
- Health check must call `store.GetStats()`.

Keep tests simple – avoid excessive test cases or middleware-specific tests.

Run: `go test ./backend/handlers -v`  
Expected: **All tests PASS** ✓

## Step 3: Main Server & E2E Tests

### 3.1 Main Server

Implement `cmd/server/main.go`:

Configuration (environment variables with defaults):

- `PORT` (default: 8080)
- `DATABASE_PATH` (default: `./data/prompts.db`)
- `LOG_FORMAT` (default: `text`) - Options: `text`, `json`
- `LOG_LEVEL` (default: `info`) - Options: `debug`, `info`, `warn`, `error`

Requirements:

- Create `./data` directory if needed.
- Initialize SQLite database at `DATABASE_PATH`.
- Initialize store and handlers.
- Serve embedded frontend (`backend/handlers/frontend.html`) at all GET requests via catch-all route.
- Mount API routes and system routes (`/api/...`, `/health`, `/metrics`).
- Use `http.Server` with graceful shutdown on SIGINT/SIGTERM.
- Log startup info (port, database path).

### 3.2 E2E Tests

Create E2E tests in `tests/e2e_test.go`:

Complete user flow:

1. Start HTTP server on a test port (e.g., `:18080`) using a temporary database file (`t.TempDir()`).
2. Create a prompt via `POST /api/prompts`: verify `201` and returned JSON has `current_version.version_number == 1`.
3. List prompts via `GET /api/prompts`: verify the new prompt is present with correct slug and title.
4. Get prompt via `GET /api/prompts/{slug}`: verify current version and contents.
5. Create new version via `POST /api/prompts/{slug}/versions`: verify `201` and `current_version.version_number == 2`.
6. List versions via `GET /api/prompts/{slug}/versions`: verify there are 2 versions with correct numbers and immutable contents.
7. Get specific versions via `GET /api/prompts/{slug}/versions/1` and `/2`: verify contents match original and new content respectively.
8. Check health via `GET /health`: verify `200` and `"database":"connected"`.
9. Check metrics via `GET /metrics`: verify counters include created prompts and versions.
10. Shutdown server cleanly.

Run: `go test ./tests -v`  
Expected: **All tests PASS** ✓

Run manually: `go run cmd/server/main.go` and verify via curl and browser.

## Step 4: Makefile

Create simple Makefile with 4 commands only:

- `run`    – Run the server (opens http://localhost:8080)
- `test`   – Run all tests
- `build`  – Build the binary
- `clean`  – Clean build artifacts and database

Requirements:

- `run`: `go run ./cmd/server`
- `test`: `go test ./...`
- `build`: `go build -o bin/prompt-registry ./cmd/server`
- `clean`: remove `bin/` and `./data/prompts.db` (and optionally `./data`)

Keep it minimal – no excessive targets.

## Step 5: Observability

### 5.1 Structured Logging Requirements

Use `log/slog` for all structured logging with configurable format and level.

**Configuration (Environment Variables):**
- `LOG_FORMAT` - `text` (default) or `json`
- `LOG_LEVEL` - `debug`, `info` (default), `warn`, or `error`

**Logging Requirements:**

1. **HTTP Request Logging** (in middleware)
   - Every request must log: `method`, `path`, `status`, `duration_ms`
   - Log level: INFO
   - Example: `time=... level=INFO msg="http request" method=GET path=/api/prompts status=200 duration_ms=5`

2. **Database Operation Logging** (in store layer)
   - Every database operation must log: `operation`, `duration_ms`, relevant context (slug, limit, etc.), `rows_returned` (for list operations)
   - Log level: INFO
   - Operations to log: `CreatePrompt`, `CreatePromptVersion`, `GetPromptBySlug`, `GetPromptVersion`, `ListPrompts`, `ListPromptVersions`, `GetStats`
   - Example: `time=... level=INFO msg="database operation" operation=CreatePrompt slug=example-prompt prompt_id=1 duration_ms=12`

3. **Error Logging**
   - All errors must include: operation context, error message, relevant identifiers
   - Log level: ERROR
   - Example: `time=... level=ERROR msg="failed to create prompt" error="prompt with slug \"example\" already exists" slug=example`

4. **Startup Logging**
   - Log configuration on startup: port, database path, base URL, log format, log level
   - Example: `time=... level=INFO msg="starting prompt registry server" port=8080 database=./data/prompts.db log_format=json log_level=info`

### 5.2 Prometheus Metrics Requirements

Implement metrics in `backend/handlers/metrics.go` using `sync/atomic` counters.

**Metrics to Track:**

1. `prompts_created_total` - Counter for total prompts created
2. `prompt_versions_created_total` - Counter for total versions created
3. `http_requests_total` - Counter for total HTTP requests
4. `http_errors_total` - Counter for total HTTP errors (4xx, 5xx)

**Export Format:**
- Plain text Prometheus format at `GET /metrics`
- Include `# HELP` and `# TYPE` comments
- Example:
```
# HELP prompts_created_total Total number of prompts created
# TYPE prompts_created_total counter
prompts_created_total 42
```

### 5.3 Health Check Requirements

Implement `GET /health` endpoint:

**Behavior:**
- Call `store.GetStats()` to verify database connectivity
- Return JSON with status and database state

**Success Response (200 OK):**
```json
{
  "status": "healthy",
  "database": "connected"
}
```

**Error Response (500 Internal Server Error):**
```json
{
  "status": "healthy",
  "database": "error"
}
```

**Logging:**
- Log health check failures at ERROR level
- Include error details in log

### 5.4 Testing Observability

**Handler Tests:**
- Verify `/health` returns correct JSON and status codes
- Verify `/metrics` returns Prometheus format
- Verify metrics increment correctly (create prompt → check counter increased)

**E2E Tests:**
- Verify `/health` endpoint returns healthy status
- Verify `/metrics` endpoint returns expected metrics

**Note:** Do not test log output directly; verify behavior via side effects.

## Step 6: Frontend (Kaizen Philosophy)

Create `backend/handlers/frontend.html` – single file embedded in Go binary, no build step.

Design Philosophy: View-based navigation where each screen does one thing well. Clean, minimal, focused on content.

### Requirements

1. **Color Scheme**
   - Black text on white/gray-50 background.
   - Gray accents only (Tailwind gray-50, gray-200, gray-300, gray-500, gray-900).
   - No other colors.
   - Header includes subtle "demo by Cleric" credit with link to https://cleric.ai

2. **URL Routing (History API)**
   - `/` – List view (homepage)
   - `/new` – Create prompt view
   - `/prompts/{slug}` – Detail/edit view
   - Clean URLs without hash fragments
   - Each view has shareable URL

3. **Views**
   - **List View**: Empty state with CTA, or grid of prompt cards. Prominent "New Prompt" button. Cards show title, description, version, updated time.
   - **Create View**: Full-height screen. Left (70%): content textarea (main focus). Right sidebar (30%): metadata form (title*, description, slug). Header with Cancel/Create buttons.
   - **Detail View**: Full-height screen. Header with back button, title, Edit button. Main area (75%): content display. Right sidebar (25%): version history (scrollable, clickable).
   - **Edit Mode**: Activated by Edit button. Content becomes editable textarea. Bottom panel shows live git-style diff (green +, red -). Actions: Cancel / Save as New Version.

4. **Technology**
   - Tailwind CSS via CDN
   - Vanilla JavaScript with History API router
   - Embedded in Go binary via `//go:embed`

5. **JavaScript Functionality**
   - History API router (pushState/popstate) handles navigation
   - List view: fetch prompts, show empty state if needed
   - Create view: POST prompt, navigate to detail on success
   - Detail view: fetch prompt + versions in parallel
   - Edit mode: compute line-by-line diff for preview
   - Version sidebar: click to preview older versions
   - Auto-refresh list every 15s
   - XSS protection via escapeHtml()

6. **Styling Principles**
   - Content-focused: large text areas, minimal chrome
   - Monospace font for prompts
   - Clean borders, subtle hover states
   - Responsive design

Test: Open http://localhost:8080, create prompt, verify clean URLs, share direct link `/prompts/slug`, test edit mode with diff preview.

## API Endpoints Summary

- `POST /api/prompts` – Create a prompt with an initial version.
- `GET /api/prompts` – List prompts with current version metadata.
- `GET /api/prompts/{slug}` – Get prompt with its current version.
- `GET /api/prompts/{slug}/versions` – List all versions for a prompt.
- `POST /api/prompts/{slug}/versions` – Create a new version for a prompt.
- `GET /api/prompts/{slug}/versions/{version}` – Get a specific prompt version.
- `GET /health` – Health check with database connectivity.
- `GET /metrics` – Prometheus metrics (`prompts_created_total`, `prompt_versions_created_total`, `http_requests_total`, `http_errors_total`).

## Key Architectural Decisions

1. **No Auth**
   - Anyone can create prompts and versions.
   - Every version is immutable and addressable by permalink.
2. **Immutable Versions**
   - New versions are append-only; previous versions are never modified or deleted.
3. **No Mocking**
   - Use real in-memory SQLite (`:memory:`) in all unit tests.
   - Use real temporary database files in E2E tests.
4. **Standard Library**
   - Use Go standard library (`net/http`, `http.ServeMux`, `httptest`) and minimal dependencies.
5. **View-Based Frontend**
   - Clean view-based navigation with History API (clean URLs without hash)
   - Each view does one thing well
   - Shareable URLs for individual prompts
   - Content-first design, minimal chrome
   - Black/white/gray colors only
   - Subtle "demo by Cleric" branding in header
6. **Structured Logging & Metrics**
   - Use `log/slog` for all logging with configurable format (text/json) and level
   - Log every HTTP request with method, path, status, duration_ms
   - Log every database operation with operation name, duration_ms, and relevant context
   - Use `sync/atomic` counters for Prometheus metrics
   - Export metrics at `/metrics` in Prometheus text format
   - Health check at `/health` verifies database connectivity
7. **Single-File Frontend**
   - No frontend build step; everything in `backend/handlers/frontend.html`
   - Embedded in binary via `//go:embed`
   - Vanilla JavaScript, no frameworks or build tools

## Step 7: Render Deployment (GitOps)

### 7.1 render.yaml Configuration

Create `render.yaml` in project root for automated Render deployment with SQLite on persistent disk.

**Service Configuration:**
```yaml
services:
  # Web service
  - type: web
    name: prompt-registry
    runtime: go
    plan: starter  # Required for persistent disk support
    region: oregon
    branch: main
    autoDeploy: true
```

**Build Command (with pre-deploy testing):**
```yaml
buildCommand: |
  # Run tests BEFORE building - deployment fails if tests fail
  go test ./... -v
  go build -o bin/prompt-registry ./cmd/server
```

**Start Command:**
```yaml
startCommand: ./bin/prompt-registry
```

**Persistent Disk (for SQLite database):**
```yaml
disk:
  name: prompt-registry-data
  mountPath: /data
  sizeGB: 1
```

**Environment Variables:**
```yaml
envVars:
  - key: PORT
    value: 8080
  - key: DATABASE_PATH
    value: /data/prompts.db  # Uses persistent disk mount
  - key: LOG_FORMAT
    value: json
  - key: LOG_LEVEL
    value: info
```

**Health Check:**
```yaml
healthCheckPath: /health
```

**Resource Allocation:**
```yaml
numInstances: 1
```

### 7.2 Deployment Requirements

**Pre-deploy Testing:**
- All tests (`go test ./...`) must pass before build
- If any test fails, deployment aborts
- Ensures broken code never reaches production

**Health Check:**
- Must respond to `GET /health` with 200 OK
- Health check verifies database connectivity via `store.GetStats()`
- Failing health check prevents deployment from going live

**Database:**
- SQLite with persistent disk (requires starter plan or higher)
- Database file stored at `/data/prompts.db` on persistent disk
- Data persists across deployments and restarts
- Schema created automatically by application on first run
- Same database technology for local, test, and production (simpler, more consistent)

**Logging:**
- Production uses JSON format (`LOG_FORMAT=json`)
- Structured logs for aggregation and analysis
- All logs visible in Render dashboard

**Auto-deploy:**
- Pushes to `main` branch trigger automatic deployment
- GitOps workflow: commit → test → build → deploy
- Rollback to previous version on failure

### 7.3 Deployment Process

1. Developer pushes code to `main` branch
2. Render detects change and starts build
3. Pre-deploy: `go test ./... -v` runs all tests
4. If tests pass: `go build -o bin/prompt-registry ./cmd/server`
5. If build succeeds: service starts with `./bin/prompt-registry`
6. Health check polls `/health` endpoint
7. If health check passes: traffic routes to new deployment
8. If any step fails: deployment aborts, rolls back to previous version

### 7.4 Monitoring and Observability

**Health Checks:**
- Continuous polling of `/health` endpoint
- Verifies application and database status
- Auto-restart on health check failures

**Metrics:**
- Prometheus metrics available at `/metrics`
- Can be scraped by external monitoring tools
- Track prompts created, versions created, HTTP requests, errors

**Logs:**
- JSON-formatted structured logs in Render dashboard
- Can export to DataDog, LogDNA, or other services
- Searchable and filterable

**Auto-restart:**
- Service automatically restarts on crashes
- Persistent disk ensures no data loss

### 7.5 Testing render.yaml

**Validate configuration:**
```bash
# Check YAML syntax
yamllint render.yaml

# Verify build command works locally
go test ./... -v && go build -o bin/prompt-registry ./cmd/server
```

**Test health check:**
```bash
# Start server
./bin/prompt-registry

# Verify health endpoint (separate terminal)
curl http://localhost:8080/health
# Should return: {"status":"healthy","database":"connected"}
```

**Test with production-like environment variables:**
```bash
PORT=8080 \
DATABASE_PATH=/tmp/test-prompts.db \
LOG_FORMAT=json \
LOG_LEVEL=info \
./bin/prompt-registry
```

## Expected Final Result

- All tests passing (store, handlers, e2e).
- Simple Makefile with 4 commands.
- Clean, minimal frontend to create and browse versioned prompts.
- Structured logs and Prometheus metrics.
- Concise README with essential information only.
- Working application: `make run` → http://localhost:8080.
- Production-ready `render.yaml` for GitOps deployment.
- Automated testing and health checks in deployment pipeline.

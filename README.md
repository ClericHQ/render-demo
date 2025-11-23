# Prompt Registry

A minimal Go application for creating and versioning prompt templates. Built following TDD principles with SQLite storage and a clean HTML/Tailwind frontend.

## Quick Start

```bash
# Run the server
make run

# Or with custom configuration
PORT=3000 DATABASE_PATH=./my.db go run ./cmd/server
```

The server will start at `http://localhost:8080`.

## Project Structure

```
/cmd/server/main.go             - Application entry point
/backend/store/store.go         - Database interface and SQLite implementation
/backend/handlers/handlers.go   - HTTP handlers with middleware
/backend/handlers/metrics.go    - Prometheus metrics tracking
/backend/models/models.go       - Data types
/web/index.html                 - Single-page frontend (no build step)
/tests/e2e_test.go              - Integration tests
/README.md                      - Essential documentation
/Makefile                       - Simple development commands
/go.mod                         - Module definition
```

## API Endpoints

### Create Prompt
```
POST /api/prompts
Content-Type: application/json

{
  "slug": "optional-slug",
  "title": "Prompt Title",
  "description": "Optional description",
  "content": "Prompt content"
}

Response: 201 Created
```

### List Prompts
```
GET /api/prompts?limit=100&offset=0

Response: 200 OK
[
  {
    "slug": "example-prompt",
    "title": "Example Prompt",
    "description": "...",
    "current_version": 2,
    "created_at": "2025-01-15T10:00:00Z",
    "updated_at": "2025-01-15T11:00:00Z"
  }
]
```

### Get Prompt
```
GET /api/prompts/{slug}

Response: 200 OK
{
  "slug": "example-prompt",
  "title": "Example Prompt",
  "description": "...",
  "current_version": {
    "version_number": 2,
    "content": "Latest content",
    "created_at": "2025-01-15T11:00:00Z"
  }
}
```

### List Versions
```
GET /api/prompts/{slug}/versions

Response: 200 OK
[
  {
    "version_number": 1,
    "content": "First version",
    "created_at": "2025-01-15T10:00:00Z"
  },
  {
    "version_number": 2,
    "content": "Second version",
    "created_at": "2025-01-15T11:00:00Z"
  }
]
```

### Create Version
```
POST /api/prompts/{slug}/versions
Content-Type: application/json

{
  "content": "New version content"
}

Response: 201 Created
```

### Get Specific Version
```
GET /api/prompts/{slug}/versions/{version}

Response: 200 OK
{
  "version_number": 1,
  "content": "Version content",
  "created_at": "2025-01-15T10:00:00Z"
}
```

### Health Check
```
GET /health

Response: 200 OK
{
  "status": "healthy",
  "database": "connected"
}
```

### Metrics
```
GET /metrics

Response: 200 OK (Prometheus text format)
```

## Database Schema

### prompts
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
```

### prompt_versions
```sql
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

## Configuration

Environment variables with defaults:

- `PORT` - Server port (default: `8080`)
- `DATABASE_PATH` - SQLite database file path (default: `./data/prompts.db`)
- `BASE_URL` - Base URL for the application (default: `http://localhost:8080`)
- `LOG_FORMAT` - Log format: `text` or `json` (default: `text`)
- `LOG_LEVEL` - Log level: `debug`, `info`, `warn`, `error` (default: `info`)

## Development Commands

```bash
# Run the server
make run

# Run all tests
make test

# Build binary
make build

# Clean build artifacts and database
make clean
```

## Observability

The application provides comprehensive observability through structured logging, Prometheus metrics, and health checks.

### Structured Logging

All logs use `log/slog` for structured, parseable output. Configure via environment variables:

```bash
# Text format (development)
LOG_FORMAT=text LOG_LEVEL=info make run

# JSON format (production)
LOG_FORMAT=json LOG_LEVEL=warn make run
```

**HTTP Request Logs:**
```
time=2025-01-15T10:00:00.000Z level=INFO msg="http request" method=GET path=/api/prompts status=200 duration_ms=5
```

**Database Operation Logs:**
```
time=2025-01-15T10:00:00.000Z level=INFO msg="database operation" operation=CreatePrompt slug=example-prompt prompt_id=1 duration_ms=12
time=2025-01-15T10:00:00.000Z level=INFO msg="database operation" operation=ListPrompts limit=100 offset=0 rows_returned=5 duration_ms=3
```

**Error Logs:**
```
time=2025-01-15T10:00:00.000Z level=ERROR msg="failed to create prompt" error="prompt with slug \"example\" already exists" slug=example
```

### Prometheus Metrics

Metrics are exposed at `GET /metrics` in Prometheus text format:

```bash
curl http://localhost:8080/metrics
```

**Available Metrics:**
- `prompts_created_total` - Counter: Total number of prompts created
- `prompt_versions_created_total` - Counter: Total number of versions created
- `http_requests_total` - Counter: Total HTTP requests received
- `http_errors_total` - Counter: Total HTTP errors (4xx, 5xx)

**Example Output:**
```
# HELP prompts_created_total Total number of prompts created
# TYPE prompts_created_total counter
prompts_created_total 42

# HELP prompt_versions_created_total Total number of prompt versions created
# TYPE prompt_versions_created_total counter
prompt_versions_created_total 87

# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total 1234

# HELP http_errors_total Total number of HTTP errors
# TYPE http_errors_total counter
http_errors_total 5
```

### Health Check

The health endpoint verifies application and database status:

```bash
curl http://localhost:8080/health
```

**Healthy Response (200 OK):**
```json
{
  "status": "healthy",
  "database": "connected"
}
```

**Unhealthy Response (500 Internal Server Error):**
```json
{
  "status": "healthy",
  "database": "error"
}
```

### Log Analysis Examples

**Find slow database operations:**
```bash
# Text format
grep "database operation" logs.txt | grep -E "duration_ms=[0-9]{3,}"

# JSON format
jq 'select(.operation != null and .duration_ms > 100)' logs.json
```

**Track error rates:**
```bash
# Text format
grep "level=ERROR" logs.txt | wc -l

# JSON format
jq 'select(.level == "ERROR")' logs.json | wc -l
```

**Monitor specific operations:**
```bash
# JSON format
jq 'select(.operation == "CreatePrompt")' logs.json
```

## Deployment

### Render (GitOps)

The application is configured for automatic deployment to Render via `render.yaml`.

**Setup:**

1. Push code to GitHub repository
2. Connect repository to Render
3. Render automatically detects `render.yaml` and creates the service
4. Deployment happens automatically on every push to `main` branch

**Key Features:**

- **Pre-deploy Testing:** Tests run before build - deployment fails if any test fails
- **Health Checks:** `/health` endpoint verifies service is running and database is connected
- **Persistent Storage:** SQLite database stored on 1GB persistent disk at `/var/data/prompts.db`
- **Auto-deploy:** Pushes to `main` branch trigger automatic deployment
- **JSON Logging:** Production uses JSON format for structured log aggregation

**Configuration:**

The service uses these environment variables in production:
- `PORT=8080` - Render's standard web port
- `DATABASE_PATH=/var/data/prompts.db` - Persistent disk mount
- `LOG_FORMAT=json` - Structured logging for production
- `LOG_LEVEL=info` - Standard production logging level
- `BASE_URL` - Set via Render dashboard (e.g., `https://prompt-registry.onrender.com`)

**Accessing Logs:**

```bash
# Via Render dashboard: Logs tab shows JSON-formatted logs
# Can be exported to external logging services (DataDog, LogDNA, etc.)
```

**Deployment Process:**

1. Developer pushes to `main` branch
2. Render detects change and starts build
3. `go test ./... -v` runs all tests
4. If tests pass: `go build -o bin/prompt-registry ./cmd/server`
5. If build succeeds: service starts with `./bin/prompt-registry`
6. Health check at `/health` must return 200 OK
7. If health check passes: traffic routes to new deployment
8. If any step fails: deployment rolls back to previous version

**Database Persistence:**

SQLite database is stored on a 1GB persistent disk mounted at `/var/data`. Data persists across deployments and restarts. Database file is automatically created on first run.

**Monitoring:**

- Health checks run continuously at `/health`
- Metrics available at `/metrics` (Prometheus format)
- Structured JSON logs in Render dashboard
- Service auto-restarts on crashes

## Technology Stack

- **Go** 1.25+
- **SQLite** (github.com/mattn/go-sqlite3)
- **Standard library** HTTP server and router
- **Tailwind CSS** (via CDN)
- **No build step** for frontend
- **Render** for hosting and GitOps deployment

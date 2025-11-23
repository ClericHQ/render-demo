.PHONY: run test build clean

run:  ## Run the server
	@echo "Starting server at http://localhost:8080"
	@go run ./cmd/server

test:  ## Run all tests
	@go test ./...

build:  ## Build the binary
	@mkdir -p bin
	@go build -o bin/prompt-registry ./cmd/server
	@echo "Binary built: bin/prompt-registry"

clean:  ## Clean build artifacts and database
	@rm -rf bin/
	@rm -f ./data/prompts.db
	@rm -rf ./data
	@echo "Cleaned build artifacts and database"

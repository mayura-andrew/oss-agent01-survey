GOLANGCI_LINT ?= golangci-lint

.PHONY: run build test vet fmt lint

run: ## Run the server (reads settings from the environment; see .env.example)
	go run ./cmd/survey

build: ## Build a static binary at ./survey
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o survey ./cmd/survey

test:
	go test -race -cover ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

lint:
	$(GOLANGCI_LINT) run

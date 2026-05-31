.PHONY: build test test-integration lint sqlc-generate

# Detect Docker daemon availability
DOCKER_OK := $(shell docker info >/dev/null 2>&1 && echo 1 || echo 0)

build:
	go build ./...

test:
	go test ./...

test-integration:
ifeq ($(DOCKER_OK),1)
	go test -tags=integration -count=1 ./...
else
	@echo "skipping integration tests: Docker daemon not running"
endif

lint:
	go vet ./...

sqlc-generate:
	go tool sqlc generate

# Makefile for lr2otelmetric
# Usage:
#   make build           # Build Go binary for host
#   make docker          # Build Docker image for linux/amd64 (default)
#   make docker PLATFORM=linux/arm64   # Build Docker image for ARM64
#   make test            # Run Go tests
#   make clean           # Remove binary

BINARY=lr2otelmetric
SRC=src/parse_vuser_log.go
DOCKER_IMAGE=lr2otelmetric
PLATFORM?=linux/amd64

build:
	cd src && go build -o ../lr2otelmetric parse_vuser_log.go

docker:
	docker build --platform=$(PLATFORM) -t $(DOCKER_IMAGE) .

test:
	cd src && go test ./...

clean:
	rm -f $(BINARY) 
# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY src/go.mod src/go.sum ./
RUN go mod download
COPY src/ .
COPY Makefile ./
RUN make build

# Final image
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/lr2otelmetric ./lr2otelmetric
COPY src/README.md ./README.md
COPY src/LICENSE ./LICENSE
ENTRYPOINT ["/app/lr2otelmetric"] 
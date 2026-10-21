# syntax=docker/dockerfile:1
FROM golang:1.27.3-alpine3.24 AS builder

ENV GOTOOLCHAIN=local

WORKDIR /src

# Download dependencies with cache mount (only for dependencies)
COPY go.mod go.sum ./
RUN --mount=type=cache,mode=0755,target=/go/pkg/mod \
    go mod download

# Add source code
COPY . .

# Full commit SHA of the workflow run, reported to Sentry as the release.
# Declared after COPY so the module download layer stays cached.
ARG SENTRY_RELEASE

# Reuse downloaded modules and compiled packages across builds.
RUN --mount=type=cache,mode=0755,target=/go/pkg/mod \
    --mount=type=cache,mode=0755,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build \
    -ldflags="-s -w -X github.com/moto-nrw/project-phoenix/cmd.release=${SENTRY_RELEASE}" \
    -o main .

# The dev tools (seed, simulate, demo) are a binary of their own (ADR 0060).
# Only the demo target builds and ships it; the production target never
# reaches this stage.
FROM builder AS devtools-builder
RUN --mount=type=cache,mode=0755,target=/go/pkg/mod \
    --mount=type=cache,mode=0755,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o devtools ./cmd/devtools

# Multi-Stage runtime base shared by the production and demo targets
FROM alpine:3.21 AS runtime
RUN apk --no-cache add ca-certificates bash tzdata
ENV TZ=Europe/Berlin

# Create non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -S appuser -u 1001 -G appgroup

WORKDIR /app
# Retrieve the binary from the previous stage
COPY --from=builder /src/main .
# Copy static template files
COPY templates templates
# Copy frontend
COPY public public

# Set ownership and switch to non-root user
RUN chown -R appuser:appgroup /app
USER appuser

# Expose port
EXPOSE 8080
# Set the binary as the entrypoint of the container
CMD ["./main", "serve"]

# Demo image: the server plus the dev tools binary for the demo-runtime sidecar.
FROM runtime AS demo
COPY --from=devtools-builder --chown=appuser:appgroup /src/devtools .

# Production image (the default target): the server binary only.
FROM runtime AS production

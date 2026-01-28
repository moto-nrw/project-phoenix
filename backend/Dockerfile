# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Download dependencies with cache mount (only for dependencies)
COPY go.mod go.sum ./
RUN --mount=type=cache,mode=0755,target=/go/pkg/mod \
    go mod download

# Add cache-busting argument
ARG CACHE_BUST=1

# Add source code
COPY . .

# Build with module cache mount to avoid re-downloading
RUN --mount=type=cache,mode=0755,target=/go/pkg/mod \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o main .

# Multi-Stage production build
FROM alpine:3.21 AS production
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
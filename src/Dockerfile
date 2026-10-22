# Multi-stage build: compile in a Go image, ship only the binary.

# Build stage: Go toolchain on Alpine, matching the version in go.mod.
FROM golang:1.26-alpine AS build
WORKDIR /src

# Copy only the module files first so the dependency download is cached
# until go.mod or go.sum change.
COPY go.mod go.sum ./
RUN go mod download

# Build a static binary (CGO_ENABLED=0) so it runs on an empty image.
# -trimpath drops local paths; -s -w strips debug info to shrink it.
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /api ./cmd/api

# Runtime stage: empty image with just the binary (no shell, no CA
# certificates, no timezone data).
FROM scratch
COPY --from=build /api /api

# The port is published in compose.yaml; EXPOSE only documents it.
EXPOSE 8080
ENTRYPOINT ["/api"]

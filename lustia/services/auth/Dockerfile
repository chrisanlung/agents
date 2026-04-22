# ---------------------------------------------------------------------------
# auth-service — multi-stage build.
#
# Build context: services/auth/ (see deploy/docker-compose.yml).
#
# common-configs resolution: the service go.mod has a host-only `replace`
# directive pointing at F:/Projects/GO/common-configs. Docker cannot reach
# outside the build context, so this image IGNORES the replace and fetches
# common-configs from the public GitHub repo via `go mod download`. This is
# the "Option C" decision documented in docs/OPERATIONS.md § Image build.
#
# Final image is Google distroless static, non-root by default (uid 65532).
# No shell, no package manager, no curl — the smallest sane runtime surface.
# Healthchecks live in compose, not here (distroless has no probe binary).
# ---------------------------------------------------------------------------

# Pin the builder to a specific minor series. Bump alongside go.mod `go 1.x`.
FROM golang:1.24-alpine AS build

WORKDIR /src

# Module graph first, so `docker build` caches dependency downloads across
# edits to source files.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Source.
COPY . .

# Build flags:
#   CGO_ENABLED=0   → fully static binary; compatible with distroless/static.
#   GOFLAGS=-trimpath → reproducible builds (no absolute host paths in binary).
#   -ldflags "-s -w" → strip symbol and DWARF info (smaller image).
ENV CGO_ENABLED=0 GOFLAGS=-trimpath GOOS=linux
RUN go build -ldflags="-s -w" -o /out/auth ./cmd/auth

# ---------------------------------------------------------------------------
# Runtime
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

# Distroless already runs as nonroot (uid 65532). Re-declare for clarity so
# anyone reading the image config sees the intent without inspecting the base.
USER nonroot:nonroot

COPY --from=build /out/auth /auth

EXPOSE 8080

ENTRYPOINT ["/auth"]

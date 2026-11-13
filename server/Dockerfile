# syntax=docker/dockerfile:1

# Stage 1: Build frontend
#
# Pinned to the build host's architecture: the output is JavaScript, identical
# for every target, so building it once per target platform would only cost
# time.
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend
WORKDIR /build
COPY web/package.json web/package-lock.json ./
# The cache mount survives between builds on a persistent builder, so a
# lockfile change re-resolves rather than re-downloads.
RUN --mount=type=cache,target=/root/.npm npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go binary
#
# Also pinned to the build host, with GOARCH taken from the target instead. Go
# cross-compiles, so a linux/arm64 image is built at native speed; leaving this
# unpinned would run the whole compile under QEMU emulation.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS backend
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Only what the compiler reads, rather than the whole context. migrations/,
# specs/ and scripts/ are not compiled — copying them here meant every
# migration invalidated this layer and rebuilt the binary.
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web.go ./
# web.go embeds all:web/dist, so the assets have to be in place before the build.
COPY --from=frontend /build/dist ./web/dist

# Declared here and not at the top of the stage: it changes with every commit,
# and the layers above stay cached.
ARG BUILD_SHA=dev
# The build cache is what makes an incremental build incremental: without it Go
# recompiles every dependency, tailscale.com included, on every run.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags "-X main.Version=${VERSION} -X main.BuildSHA=${BUILD_SHA}" -o freereps ./cmd/freereps

# Stage 3: Runtime
FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /build/freereps .
# Read at startup, not embedded — so they join the image after the build and a
# migration-only change reuses the compiled binary layer.
COPY migrations ./migrations
# Port 8080 for dev mode (tailscale.enabled: false); tsnet mode uses port 80 via tailnet
EXPOSE 8080
ENTRYPOINT ["./freereps"]

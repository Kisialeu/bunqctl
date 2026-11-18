# syntax=docker/dockerfile:1.7
#
# Builds bunqctl in a container. Stages you can pick with --target:
#
#   artifacts   only the binary, to copy out of the build:
#               docker build --target artifacts --output type=local,dest=dist .
#   test        gofmt, go vet and the unit tests
#   (default)   a small Linux image for --help, --version and "docs".
#
# bunqctl keeps its keys in the macOS Keychain, so use it for real on your Mac:
# build the macOS binary with GOOS=darwin (see docs/building.txt).

ARG GO_VERSION=1.27.4
ARG ALPINE_VERSION=3.23

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS base
WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY docs ./docs

# -race needs cgo and a C compiler, so run `go test -race ./...` on your machine.
FROM base AS test
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    unformatted="$(gofmt -l .)"; \
    if [ -n "$unformatted" ]; then echo "gofmt needed:" "$unformatted"; exit 1; fi; \
    CGO_ENABLED=0 go vet ./... && \
    CGO_ENABLED=0 go test ./...

FROM base AS build
ARG TARGETOS
ARG TARGETARCH
# Set GOOS and GOARCH to build for another system than the image,
# for example --build-arg GOOS=darwin --build-arg GOARCH=arm64.
ARG GOOS
ARG GOARCH
ARG VERSION=dev
ARG COMMIT=unknown

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS="${GOOS:-${TARGETOS}}" GOARCH="${GOARCH:-${TARGETARCH}}" \
    go build -trimpath \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -o /out/bunqctl ./cmd/bunqctl

FROM scratch AS artifacts
COPY --from=build /out/bunqctl /bunqctl

FROM scratch

COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=base /etc/passwd /etc/passwd
COPY --from=base /etc/group /etc/group
COPY --from=build /out/bunqctl /bunqctl

USER 65532:65532
STOPSIGNAL SIGTERM

ENTRYPOINT ["/bunqctl"]

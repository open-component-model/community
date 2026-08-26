# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.26@sha256:dc2521c2a906db43073b8b4d99f491b6341cf15610b6ebbab187c45153f9959e AS builder

WORKDIR /workspace
RUN go env -w GOMODCACHE=/root/.cache/go-build

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

# Copy the go source
COPY cmd/ cmd/
COPY compver/ compver/
COPY helmvalues/ helmvalues/

ARG TARGETOS
ARG TARGETARCH

RUN mkdir bin

FROM builder AS ocm-kit-builder
# VERSION is embedded into the binary (exposed via `ocm-kit --version`).
ARG VERSION=dev
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w -X main.version=${VERSION}" -a -o bin/ocm-kit ./cmd/ocm-kit

# Use distroless as minimal base image to package the binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot@sha256:963fa6c544fe5ce420f1f54fb88b6fb01479f054c8056d0f74cc2c6000df5240 AS ocm-kit
WORKDIR /
COPY --from=ocm-kit-builder /workspace/bin/ocm-kit .
USER 65532:65532
ENTRYPOINT ["/ocm-kit"]

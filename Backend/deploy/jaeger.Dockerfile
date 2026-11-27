# Jaeger's minimal runtime lacks wget; add a static BusyBox for its probe.
FROM busybox:1.37.0-musl AS probe
RUN mkdir -p /badger/data /badger/key

# Match the UI submodule pinned by Jaeger v1.67.0:
# jaegertracing/jaeger-ui@eece89037f34e439b50e7bb068a9360c437bc6ba.
# The Go module archive omits built UI assets, so supply them before go:embed.
FROM alpine:3.23 AS ui
RUN apk add --no-cache ca-certificates curl
WORKDIR /ui
RUN curl --fail --location --retry 3 \
      https://github.com/jaegertracing/jaeger-ui/releases/download/v1.67.0/assets.tar.gz \
      --output /tmp/assets.tar.gz \
    && echo 'a74186f0ef15fb5a5e2070b04b618b17acf4d2de96c565da179925a858417305  /tmp/assets.tar.gz' | sha256sum -c - \
    && tar -xzf /tmp/assets.tar.gz --strip-components=3 \
    && test -s index.html \
    && find . -type f -exec gzip -n -9 {} \;

# Keep the existing Badger format while avoiding quadratic cache preloading on
# restart: the upstream reader returns one service name per persisted span.
FROM golang:1.27.3-alpine AS build
RUN apk add --no-cache patch
WORKDIR /src
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download github.com/jaegertracing/jaeger@v1.67.0 \
    && cp -R /go/pkg/mod/github.com/jaegertracing/jaeger@v1.67.0/. /src/ \
    && chmod -R u+w /src
COPY jaeger-prefill.patch /tmp/jaeger-prefill.patch
RUN patch -p1 < /tmp/jaeger-prefill.patch
COPY --from=ui /ui/ /src/cmd/query/app/ui/actual/
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    test -s cmd/query/app/ui/actual/index.html.gz \
    && CGO_ENABLED=0 go test ./internal/storage/v1/badger/spanstore ./cmd/query/app/ui \
    && CGO_ENABLED=0 go build -o /out/all-in-one-linux ./cmd/all-in-one

FROM jaegertracing/all-in-one:1.67.0
COPY --from=build /out/all-in-one-linux /go/bin/all-in-one-linux
COPY --from=probe /bin/busybox /bin/busybox
COPY --from=probe --chown=10001:10001 /badger /badger

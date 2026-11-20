# syntax=docker/dockerfile:1
FROM golang:1.27.3-alpine AS plugins
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12 \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2

FROM bufbuild/buf:1.75.0
COPY --from=plugins /go/bin/protoc-gen-go /usr/local/bin/protoc-gen-go
COPY --from=plugins /go/bin/protoc-gen-go-grpc /usr/local/bin/protoc-gen-go-grpc
WORKDIR /work
ENTRYPOINT ["buf"]

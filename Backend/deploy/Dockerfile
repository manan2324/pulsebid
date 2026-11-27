# syntax=docker/dockerfile:1
FROM golang:1.27.3-alpine AS build
ARG SERVICE=bid-gateway
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    case "$SERVICE" in bid-gateway|campaign-service|event-aggregator|mock-bidder|seed|migrate) ;; *) echo "Unknown SERVICE: $SERVICE" >&2; exit 1 ;; esac \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/service "./cmd/$SERVICE" \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/healthcheck ./cmd/healthcheck

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/service /service
COPY --from=build /out/healthcheck /healthcheck
USER nonroot:nonroot
ENTRYPOINT ["/service"]

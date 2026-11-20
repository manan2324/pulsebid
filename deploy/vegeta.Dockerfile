FROM golang:alpine AS builder
RUN go install github.com/tsenart/vegeta/v12@latest

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /go/bin/vegeta /usr/local/bin/vegeta
ENTRYPOINT ["vegeta"]

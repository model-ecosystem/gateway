FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY sse_server.go .
RUN go mod init sse-server && \
    go build -o sse_server sse_server.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/sse_server /app/sse_server
EXPOSE 3010
CMD ["/app/sse_server", "-addr", ":3010"]
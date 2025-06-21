FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY echo_server.go .
RUN go mod init echo-server && \
    go get github.com/gorilla/websocket && \
    go build -o echo_server echo_server.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/echo_server /app/echo_server
EXPOSE 3001
CMD ["/app/echo_server", "-addr", ":3001"]
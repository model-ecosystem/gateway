FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY main.go .
RUN go mod init test-server && go build -o server main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
EXPOSE 3000
CMD ["./server"]
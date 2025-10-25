FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Build server binary by default
RUN CGO_ENABLED=0 GOOS=linux go build -o servify ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/servify .
# Config file can be mounted as ./config.yml at runtime

EXPOSE 8080

CMD ["./servify"]

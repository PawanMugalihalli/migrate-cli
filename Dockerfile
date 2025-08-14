# Stage 1: Build binary
FROM golang:1.24.6-alpine AS builder

WORKDIR /app
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o migrate main.go

# Stage 2: Runtime
FROM alpine:latest
WORKDIR /app

COPY --from=builder /app/migrate /usr/local/bin/migrate

ENTRYPOINT ["migrate"]

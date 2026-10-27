# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk update && apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /docupilot-backend .

# Final minimal runtime
FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata libreoffice

COPY --from=builder /docupilot-backend /app/docupilot-backend

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["/app/docupilot-backend"]

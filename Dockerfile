# Stage 1: Build statically compiled Go binary
FROM golang:1.21-alpine AS builder

# Install CA certificates for TLS handshakes and tzdata for time operations
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy source files
COPY go.mod ./
COPY main.go ./
COPY static ./static
COPY templates ./templates

# Compile the Go application statically
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o server .

# Stage 2: Production release stage
FROM registry.access.redhat.com/ubi9/ubi-micro:latest

# Copy CA certificates & timezone data from builder to ensure safe outbound HTTPS and accurate time logging
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

WORKDIR /app

# Copy the executable and static templates
COPY --from=builder /app/server /app/server
COPY --from=builder /app/templates /app/templates
COPY --from=builder /app/static /app/static

# Adhere to Production Grade security: run the container as a numeric non-root user (UID 10001)
USER 10001:10001

# Set standard environment properties
ENV PORT=8080
EXPOSE 8080

# Execute server binary
ENTRYPOINT ["/app/server"]

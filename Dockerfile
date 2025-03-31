# Build stage
FROM golang:1.23-alpine AS builder

# Install only the necessary build dependencies
RUN apk add --no-cache git build-base

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application with flags to reduce binary size
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o yt-fabric-ui .

# Fabric tool build stage
FROM golang:1.23-alpine AS fabric-builder

# Install git for go install
RUN apk add --no-cache git

# Install the fabric CLI tool
RUN go install github.com/danielmiessler/fabric@latest

# Final stage - using scratch for minimal size
FROM alpine:3.19

# Install only the runtime dependencies needed
RUN apk add --no-cache ca-certificates && \
    # Create fabric user with UID 1000
    adduser -D -u 1000 fabric && \
# Create necessary directories
    mkdir -p /app/data/videos /app/web/templates /home/fabric/.config/fabric && \
    # Set proper permissions
    chown -R fabric:fabric /home/fabric /app

# Copy the built binary from the builder stage
COPY --from=builder /app/yt-fabric-ui /app/
COPY --from=builder /app/web/templates /app/web/templates

# Copy the fabric CLI tool from the fabric-builder stage
COPY --from=fabric-builder /go/bin/fabric /usr/local/bin/

# Set working directory
WORKDIR /app

# Expose the port the app runs on
EXPOSE 8085

# Switch to fabric user
USER fabric

# Command to run the application
CMD ["./yt-fabric-ui", "--port", "8085"]

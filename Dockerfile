# syntax=docker/dockerfile:1.7

# Build stage
FROM golang:1.24-alpine AS builder

# Install only the necessary build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

# Copy the source code
COPY . .

# Build the application with flags to reduce binary size and metadata leakage
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o yt-fabric-ui .

# Fabric tool build stage
# Fabric >= v1.4.415 requires Go >= 1.25.1, so use Go 1.25 for this stage.
FROM golang:1.25-alpine AS fabric-builder

# Install git for go install
RUN apk add --no-cache git

# Install the fabric CLI tool
ARG FABRIC_VERSION=latest
RUN go install github.com/danielmiessler/fabric/cmd/fabric@${FABRIC_VERSION}

# Final stage
FROM alpine:3.20

# Install only the runtime dependencies needed
RUN apk add --no-cache ca-certificates && \
    # Create fabric user with UID 1000
    adduser -D -u 1000 -h /home/fabric fabric && \
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

# Runtime defaults
ENV HOME=/home/fabric

# Expose the port the app runs on
EXPOSE 8090

# Switch to fabric user
USER fabric

# Command to run the application
CMD ["./yt-fabric-ui", "--port", "8090"]

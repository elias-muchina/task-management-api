# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies first
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/api

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/main .

# Create app directory and copy .env
RUN mkdir -p /app
COPY .env /app/.env

# Expose port
EXPOSE 8080

# Run the application
CMD ["./main"]
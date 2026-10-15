# Build stage
FROM golang:1.26-alpine AS builder 

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build clean static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/vali .

# Production stage
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy binary from builder
COPY --from=builder /bin/vali /app/vali

# Default environment flags
ENV VALI_PORT=8080

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/vali"]

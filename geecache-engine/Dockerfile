# Build the cache binary.
FROM golang:1.25-alpine AS builder
WORKDIR /app
# Override GOPROXY at build time when a regional mirror is needed.
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
# Cache dependencies separately from source changes.
COPY go.mod go.sum ./
RUN go mod download
# Copy the module source.
COPY . .
# Build the geecache-server executable.
RUN CGO_ENABLED=0 go build -buildvcs=false -o geecache-server ./cmd/simplecache

# Create the runtime image.
FROM alpine:latest
WORKDIR /app
# Copy the compiled executable from the builder.
COPY --from=builder /app/geecache-server .
# Default command.
CMD ["./geecache-server"]

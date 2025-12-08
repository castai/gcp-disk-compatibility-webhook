# Build stage
FROM golang:1.25-alpine AS builder
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

ENV CGO_ENABLED=0
ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}

ENV GOCACHE=/go-cache
ENV GOMODCACHE=/gomod-cache

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY main.go main.go
COPY pkg/ pkg/

# Build
RUN go build -a -o webhook main.go

# Runtime stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/webhook .

USER 65532:65532

ENTRYPOINT ["/webhook"]

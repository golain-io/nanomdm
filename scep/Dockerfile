# Build SCEP server from source (supports amd64 and arm64; no linux-arm64 pre-built release)
FROM golang:1.22-alpine AS builder
ARG TARGETARCH=amd64
ARG SCEP_VERSION=v2.3.0
RUN apk add --no-cache git
WORKDIR /src
RUN git clone --depth 1 --branch ${SCEP_VERSION} https://github.com/micromdm/scep.git . \
    && CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -o /out/scepserver ./cmd/scepserver

FROM alpine:3.18
RUN apk add --no-cache ca-certificates && rm -rf /var/cache/apk/*
WORKDIR /app
COPY --from=builder /out/scepserver /app/scepserver
RUN chmod +x /app/scepserver && mkdir -p /app/depot
EXPOSE 8080
ENTRYPOINT ["/app/scepserver"]
CMD ["-allowrenew", "0", "-challenge", "nanomdm", "-debug", "-port", "8080", "-depot", "/app/depot"]

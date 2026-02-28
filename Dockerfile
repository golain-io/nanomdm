FROM golang:1.22 AS builder

WORKDIR /go/app

# TARGETARCH is set by Docker Buildx (e.g. amd64, arm64). Default for local build.
ARG TARGETARCH=amd64

COPY . .

RUN ARCH=${TARGETARCH:-amd64} && \
    CGO_ENABLED=0 make \
    nanomdm-linux-$$ARCH \
    nano2nano-linux-$$ARCH

FROM gcr.io/distroless/static

WORKDIR /app

ARG TARGETARCH=amd64

COPY --from=builder /go/app/nanomdm-linux-${TARGETARCH} /app/nanomdm
COPY --from=builder /go/app/nano2nano-linux-${TARGETARCH} /app/nano2nano

COPY scep_data/ca.pem /app/ca.pem
COPY scep_data/ca.key /app/ca.key

EXPOSE 9000

VOLUME ["/app/dbkv", "/app/db"]

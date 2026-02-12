FROM golang:1.22 AS builder

WORKDIR /go/app

COPY . .

RUN CGO_ENABLED=0 make \
    nanomdm-linux-amd64 \
    nano2nano-linux-amd64

FROM gcr.io/distroless/static

WORKDIR /app

COPY --from=builder /go/app/nanomdm-linux-amd64 /app/nanomdm
COPY --from=builder /go/app/nano2nano-linux-amd64 /app/nano2nano

COPY scep_data/ca.pem /app/ca.pem
COPY scep_data/ca.key /app/ca.key

EXPOSE 9000

VOLUME ["/app/dbkv", "/app/db"]

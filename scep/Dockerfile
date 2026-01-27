FROM alpine:3.18

RUN apk add --no-cache \
    ca-certificates \
    wget \
    && rm -rf /var/cache/apk/*

WORKDIR /app

ARG SCEP_VERSION=v2.3.0
ARG SCEP_ARCH=linux-amd64

RUN apk add --no-cache unzip curl

RUN curl -RLO https://github.com/micromdm/scep/releases/download/${SCEP_VERSION}/scepserver-${SCEP_ARCH}-${SCEP_VERSION}.zip \
    && unzip scepserver-${SCEP_ARCH}-${SCEP_VERSION}.zip \
    && chmod +x scepserver-${SCEP_ARCH} \
    && mkdir -p /app/depot
    # && ./scepserver-${SCEP_ARCH} ca -init \
    # && pwd \
    # && ls -la

RUN ls -la /app/depot

EXPOSE 8080

ENTRYPOINT ["/app/scepserver-linux-amd64"]
CMD ["-allowrenew", "0", "-challenge", "nanomdm", "-debug", "-port", "8080", "-depot", "/app/depot" ]

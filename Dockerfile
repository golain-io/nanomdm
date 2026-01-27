FROM gcr.io/distroless/static

WORKDIR /app

COPY nanomdm-linux-amd64-static /app/nanomdm
COPY nano2nano-linux-amd64-static /app/nano2nano

COPY scep_data/ca.pem /app/ca.pem
COPY scep_data/ca.key /app/ca.key

EXPOSE 9000

VOLUME ["/app/dbkv", "/app/db"]

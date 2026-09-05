# notifications — build multi-stage (Devy golden path)
FROM golang:1.26-bookworm AS build
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/*
# Módulos Go privados (go-shared, mercadocercano/*): el CI inyecta GITHUB_TOKEN como
# build-arg (PLAT-E35 T2). Sin token, go mod download del módulo privado falla — igual
# patrón que iam-service.
ARG GITHUB_TOKEN
ENV GOPRIVATE=github.com/hornosg/*,github.com/mercadocercano/*
RUN if [ -n "$GITHUB_TOKEN" ]; then git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"; fi
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /app/server ./src

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -r -u 10001 -m app
WORKDIR /app
COPY --from=build /app/server /app/server
USER app
EXPOSE 8282
ENTRYPOINT ["/app/server"]

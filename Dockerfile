# notifications — build multi-stage (Devy golden path)
FROM golang:1.25-bookworm AS build
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/*
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

ARG GO_VERSION=1.25
ARG TARGET=sdgen-api

FROM golang:${GO_VERSION}-alpine AS builder

ARG TARGET

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    git \
    build-base

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=1 GOOS=linux
RUN go build -ldflags="-s -w" -o /out/app ./cmd/${TARGET}

FROM alpine:3.20

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    sqlite-libs

WORKDIR /app

COPY --from=builder /out/app /app/app

COPY internal/web/templates /app/internal/web/templates
COPY scenarios /app/scenarios

ENV SDGEN_SCENARIOS_DIR=/app/scenarios \
    SDGEN_RUNS_DB=/data/runs.db \
    SDGEN_LOG_LEVEL=info \
    SDGEN_BIND=:8080 \
    PORT=8080

EXPOSE 8080

RUN addgroup -S app && adduser -S app -G app
RUN mkdir -p /data && chown -R app:app /data
USER app

ENTRYPOINT ["/app/app"]

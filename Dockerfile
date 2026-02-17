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
COPY targets /app/targets

ENV SDGEN_SCENARIOS_DIR=/app/scenarios \
    SDGEN_TARGETS_DIR=/app/targets \
    SDGEN_RUNS_DB=/app/sdgen-runs.sqlite \
    SDGEN_LOG_LEVEL=info \
    SDGEN_BIND_ADDR=:8080 \
    PORT=8080

EXPOSE 8080

RUN addgroup -S app && adduser -S app -G app
USER app

ENTRYPOINT ["/app/app"]


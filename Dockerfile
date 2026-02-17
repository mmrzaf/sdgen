ARG GO_VERSION=1.25
ARG TARGET=sdgen-api

# ---------- build ----------
FROM golang:${GO_VERSION}-alpine AS builder
ARG TARGET

RUN apk add --no-cache git build-base

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${TARGET}

# ---------- runtime ----------
FROM scratch

WORKDIR /app

COPY --from=builder /out/app /app/app

COPY --from=builder /src/internal/web/templates /app/internal/web/templates
COPY --from=builder /src/scenarios /app/scenarios

ENV SDGEN_SCENARIOS_DIR=/app/scenarios \
    SDGEN_LOG_LEVEL=info \
    SDGEN_BIND=:8080 \
    PORT=8080 \
    TZ=Etc/UTC

EXPOSE 8080

USER 65532:65532

ENTRYPOINT ["/app/app"]


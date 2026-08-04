ARG GO_VERSION=1.25.12
ARG BUILDER_ALPINE_VERSION=3.24
ARG ALPINE_VERSION=3.22.5

FROM golang:${GO_VERSION}-alpine${BUILDER_ALPINE_VERSION} AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/processing \
    ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/migrate \
    ./cmd/migrate

FROM alpine:${ALPINE_VERSION} AS runtime-base

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app

USER app

FROM runtime-base AS migrator

COPY --from=builder --chown=app:app /out/migrate /app/migrate
COPY --chown=app:app migrations /app/migrations

ENTRYPOINT ["/app/migrate"]

FROM runtime-base AS app

COPY --from=builder --chown=app:app /out/processing /app/processing

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/app/processing"]

FROM golang:1.24.1-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/server ./cmd/server \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/worker ./cmd/worker \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/migrate ./cmd/migrate \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/seed ./cmd/seed

FROM alpine:3.21

RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app \
    && mkdir -p /app/data/uploads \
    && chown -R app:app /app
WORKDIR /app

COPY --from=build --chown=app:app /out/server /app/server
COPY --from=build --chown=app:app /out/worker /app/worker
COPY --from=build --chown=app:app /out/migrate /app/migrate
COPY --from=build --chown=app:app /out/seed /app/seed
COPY --from=build /src/migrations /app/migrations

ENV APP_ENV=prod \
    HTTP_ADDR=:8080 \
    RUN_WORKER=false \
    UPLOAD_DIR=/app/data/uploads
EXPOSE 8080
VOLUME ["/app/data/uploads"]
USER app

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/health/live || exit 1
CMD ["/app/server"]

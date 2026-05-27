# syntax=docker/dockerfile:1

# ---- build stage -----------------------------------------------------------
# mattn/go-sqlite3 is a cgo wrapper, so we need a C toolchain on the build
# image. Alpine keeps the final image small.
FROM golang:1.25-alpine AS build

RUN apk add --no-cache gcc musl-dev
WORKDIR /src

# Cache go.mod / go.sum layer so module downloads don't re-run on every
# source change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO is required by mattn/go-sqlite3.
# -ldflags trims the symbol table for a smaller binary.
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /out/forum ./cmd/server


# ---- runtime stage ---------------------------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates \
 && adduser -D -u 1000 forum \
 && mkdir -p /data \
 && chown forum:forum /data

WORKDIR /app
COPY --from=build /out/forum /app/forum
COPY --from=build /src/web /app/web

# Ship the seeded database inside the image. The entrypoint copies it into
# the runtime volume only when the volume is empty, so subsequent starts
# preserve whatever the user has written. `docker compose down -v` wipes
# the volume and the next start re-seeds from /seed/forum.db.
COPY forum.db /seed/forum.db

USER forum

# SQLite lives in a volume so data survives container rebuilds. Override
# FORUM_DB at runtime if you want a different path.
ENV FORUM_DB=/data/forum.db \
    FORUM_ADDR=:8080
VOLUME ["/data"]
EXPOSE 8080

# Seed-then-exec: copy the bundled DB into the volume on first start, then
# replace this shell with the server so signals reach PID 1 cleanly.
CMD ["sh", "-c", "[ -f /data/forum.db ] || cp /seed/forum.db /data/forum.db; exec /app/forum"]

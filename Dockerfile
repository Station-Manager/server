# builder stage
FROM golang:1.25-alpine AS builder
WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -ldflags="-s -w" -o /app/server ./main.go

# final stage with Postgres + webserver
FROM alpine:3.18

# Install Postgres and CA certificates for TLS-aware clients
RUN apk add --no-cache ca-certificates postgresql postgresql-contrib su-exec bash

WORKDIR /app

# Copy the server binary and config
COPY --from=builder /app/server ./server
COPY --from=builder /app/config.json ./config.json

# Create postgres data directory and app user
RUN mkdir -p /var/lib/postgresql/data /run/postgresql && \
    chown -R postgres:postgres /var/lib/postgresql /run/postgresql && \
    addgroup -S app && adduser -S -G app app && \
    chown -R app:app /app

# Environment for Postgres and app
ENV PGDATA=/var/lib/postgresql/data \
    PGPORT=5432 \
    POSTGRES_DB=station_manager \
    POSTGRES_USER=smuser \
    POSTGRES_PASSWORD=1q2w3e4r \
    PORT=3000

EXPOSE 3000 5432

# Simple entrypoint script to init and start Postgres, then the web server
COPY <<'EOF' /app/entrypoint.sh
#!/usr/bin/env bash
set -euo pipefail

# Initialize database if empty
if [ ! -s "$PGDATA/PG_VERSION" ]; then
  echo "Initializing PostgreSQL data directory at $PGDATA"
  su-exec postgres initdb -D "$PGDATA"

  # Configure PostgreSQL to listen on all interfaces and trust local connections
  echo "listen_addresses = '*'" >> "$PGDATA/postgresql.conf"

  # Start postgres for bootstrap
  su-exec postgres pg_ctl -D "$PGDATA" -w start

  # Create user and database if they don't exist (simple, robust SQL)
  psql --username=postgres <<SQL
SELECT 'create role $POSTGRES_USER login password ''$POSTGRES_PASSWORD''' \
  WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '$POSTGRES_USER')\gexec

SELECT 'create database $POSTGRES_DB owner $POSTGRES_USER' \
  WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$POSTGRES_DB')\gexec
SQL

  # Stop bootstrap server
  su-exec postgres pg_ctl -D "$PGDATA" -w stop
fi

# Start PostgreSQL in the background
su-exec postgres postgres -D "$PGDATA" -p "$PGPORT" &

# Wait briefly for Postgres to accept connections
for i in {1..10}; do
  if pg_isready -q -h 127.0.0.1 -p "$PGPORT"; then
    break
  fi
  echo "Waiting for PostgreSQL to become ready... ($i)"
  sleep 1
done

# Export DB connection environment expected by the app, if any
export SM_DEFAULT_DB=pg

# Finally, run the web server as app user
exec su-exec app /app/server
EOF

RUN chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]

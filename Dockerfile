# builder stage
FROM golang:1.25-alpine AS builder
WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -ldflags="-s -w" -o /app/server .

# final stage
FROM alpine:3.18
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/server .

RUN addgroup -S app && adduser -S -G app app && chown -R app:app /app
USER app

EXPOSE 3000
ENV PORT=3000

ENTRYPOINT ["/app/server"]

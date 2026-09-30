# ---- build stage ----
FROM golang:1.27-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG BUILD_TIME
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w -X main.BuildTime=${BUILD_TIME}" \
    -o /out/bot ./cmd/bot

# ---- runtime stage ----
FROM alpine:3.20

# Российский корневой сертификат. API MAX отдаёт цепочку от Russian Trusted Sub CA,
# а в стандартном бандле alpine этого корня нет: без него TLS падает с
# "x509: certificate signed by unknown authority" и бот уходит в бесконечный
# рестарт — выглядит как «запустился, но не отвечает».
# Sub CA приходит в цепочке с сервера, поэтому достаточно корня.
COPY deploy/certs/ /usr/local/share/ca-certificates/

RUN apk add --no-cache ca-certificates tzdata \
    && update-ca-certificates \
    && adduser -D -u 10001 bot

WORKDIR /app

COPY --from=builder /out/bot /app/bot
COPY --from=builder /src/migrations /app/migrations

ENV APP_ENV=prod
ENV DB_MIGRATION_PATH=/app/migrations

USER bot

ENTRYPOINT ["/app/bot"]

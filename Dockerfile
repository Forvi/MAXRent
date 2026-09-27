# ---- build stage ----
FROM golang:1.27-alpine AS builder

WORKDIR /src

# Зависимости отдельным слоем для кеширования
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Сборка
ARG BUILD_TIME
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w -X main.BuildTime=${BUILD_TIME}" \
    -o /out/bot ./cmd/bot

# ---- runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 bot

WORKDIR /app

COPY --from=builder /out/bot /app/bot
COPY --from=builder /src/migrations /app/migrations

ENV APP_ENV=prod

USER bot

# Healthcheck
ENTRYPOINT ["/app/bot"]

# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src

COPY go.mod ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go mod tidy && go build -o /out/kuhnya ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates wget

# Создаём непривилегированного пользователя
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Копируем бинарник и устанавливаем владельца
COPY --from=build /out/kuhnya /usr/local/bin/kuhnya
RUN chown appuser:appgroup /usr/local/bin/kuhnya && chmod 755 /usr/local/bin/kuhnya

USER appuser
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/kuhnya"]
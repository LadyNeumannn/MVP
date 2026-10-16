# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src

COPY go.mod ./
COPY . .
RUN go build -o /out/restaurant-example ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates

# Создаём непривилегированного пользователя
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=build /out/restaurant-example /usr/local/bin/restaurant-example
RUN chown appuser:appgroup /usr/local/bin/restaurant-example && chmod 755 /usr/local/bin/restaurant-example

USER appuser
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/restaurant-example"]
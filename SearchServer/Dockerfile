FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/search-service

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/search-service .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENV KAFKA_BROKERS="kafka:9092" \
    HTTP_PORT=8080
EXPOSE 8080
CMD ["./search-service"]

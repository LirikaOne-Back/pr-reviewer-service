FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/service ./cmd

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/service .
COPY migrations ./migrations

EXPOSE 8080

CMD ["./service"]

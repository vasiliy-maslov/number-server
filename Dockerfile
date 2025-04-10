FROM golang:1.24 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o number-server main.go models.go server.go worker.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/number-server .
COPY config.json .
EXPOSE 8080
CMD ["./number-server"]
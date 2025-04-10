# Базовый образ с Go
FROM golang:1.22 AS builder

# Рабочая директория в контейнере
WORKDIR /app

# Копируем go.mod и go.sum для загрузки зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь код
COPY . .

# Собираем приложение
RUN go build -o number-server main.go models.go server.go worker.go

# Финальный образ (меньше размер, без Go)
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/number-server .
COPY config.json .

# Порт, который будет открыт
EXPOSE 8080

# Команда для запуска
CMD ["./number-server"]
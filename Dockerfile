# --- Этап 1: Сборка приложения ---
FROM golang:1.25-alpine AS builder

# Устанавливаем необходимые системные утилиты
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Кэшируем зависимости Go
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Компилируем бинарник в статическом виде (без динамических линковок)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o gateway ./cmd/gateway/main.go

# --- Этап 2: Финальный минимальный образ ---
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Копируем скомпилированный бинарник из предыдущего этапа
COPY --from=builder /app/gateway .
# Если у вас есть конфигурационные файлы (например, config.yaml), копируем их
# COPY --from=builder /app/config.yaml . 

# Открываем порт, который слушает ваш гейтвей (например, 8080)
EXPOSE 8080

# Запуск приложения
ENTRYPOINT ["./gateway"]

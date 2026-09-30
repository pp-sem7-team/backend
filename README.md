# Backend

Backend проекта для сопровождения Git workflow и накопления контекста разработки.

## Разработка

### Локальный запуск

Скопируйте файл окружения:
```bash
cp .env.local.example .env
```

Запустите сервер:
```bash
go run ./cmd/server
```

### Запуск через Docker Compose

Скопируйте файл окружения:
```bash
cp .env.docker.example .env
```

Запустите сервисы:
```bash
docker compose up --build
```

### Доступ

Сервер: http://localhost:8080 \
Swagger UI: http://localhost:8080/swagger/index.html


## Swagger

Для генерации документации после изменения Swagger-аннотаций:
```bash
swag init -g cmd/server/main.go
```

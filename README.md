# ws-messenger

WebSocket мессенджер на Go + PostgreSQL.

## Стек
- Go 1.25.6
- WebSocket: nhooyr.io/websocket
- HTTP: chi
- DB: pgx + PostgreSQL
- JWT auth

## Запуск (локально)
1) Скопируй `.env.example` в `.env` и обнови значения.
2) Запусти инфраструктуру:

```bash
docker-compose up -d
```

3) Прогони миграции (через migrate):

```bash
migrate -path ./migrations -database "postgres://ws:ws@localhost:5432/ws_messenger?sslmode=disable" up
```

4) Запусти сервис:

```bash
go run ./cmd/server
```

## API
Swagger (swaggo):

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/server/main.go -d ./cmd,./internal -o api/docs
```

Swagger UI: `/swagger/index.html`.

Основные эндпоинты:
- POST `/auth/register`
- POST `/auth/login`
- POST `/chats`
- GET `/chats`
- POST `/chats/join/{code}`
- GET `/chats/{id}/messages`
- GET `/ws/chats/{id}` (WebSocket)

## WebSocket сообщения
Входящее:
```json
{"type":"message","content":"hello"}
```

Исходящее:
```json
{"type":"message","data":{"id":"...","chat_id":"...","user_id":"...","content":"...","created_at":"..."}}
```

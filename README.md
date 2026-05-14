# api-gateway-svc

HTTP API для lite-chat demo.

## Run

```bash
HTTP_ADDR=:8080 MESSAGES_GRPC_ADDR=localhost:9999 TOKEN_SECRET=dev-secret go run ./cmd
```

## API

- `POST /api/auth/login` with `{ "username": "bobik1", "password": "aboba" }`
- `GET /api/me`
- `GET /api/chats`
- `POST /api/chats/private` with `{ "username": "bobik2" }`
- `GET /api/chats/{chat_id}/messages`
- `POST /api/chats/{chat_id}/messages` with `{ "body": "hello" }`

Protected endpoints require `Authorization: Bearer <token>`.

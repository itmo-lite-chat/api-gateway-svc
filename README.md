# api-gateway-svc

HTTP API для lite-chat demo.

## Run

```bash
docker compose up -d postgres mongodb
(cd ../users-svc && env $(cat dev/.env.example | xargs) go run ./cmd)
(cd ../chats-svc && env $(cat dev/.env.example | xargs) go run ./cmd)
(cd ../messages-svc && env $(cat dev/.env | xargs) go run ./cmd)
HTTP_ADDR=:8080 USERS_GRPC_ADDR=localhost:9991 CHATS_GRPC_ADDR=localhost:9992 MESSAGES_GRPC_ADDR=localhost:9990 TOKEN_SECRET=dev-secret go run ./cmd
```

## API

- `POST /api/auth/login` with `{ "username": "bobik1", "password": "aboba" }`
- `GET /api/me`
- `GET /api/chats`
- `POST /api/chats/private` with `{ "username": "bobik2" }`
- `GET /api/chats/{chat_id}/messages`
- `POST /api/chats/{chat_id}/messages` with `{ "body": "hello" }`

Protected endpoints require `Authorization: Bearer <token>`.

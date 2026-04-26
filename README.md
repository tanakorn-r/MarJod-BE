# Finance Chat

Chat-based income/expense tracker powered by Ollama (local LLM).

## Architecture

```
main.go
├── config/        — singleton app config (env vars)
├── database/      — singleton GORM + SQLite connection
├── model/         — Transaction model
├── repository/    — DB access layer
├── service/       — business logic + Ollama integration
├── controller/    — HTTP handlers (Gin)
└── router/        — route wiring + DI
```

## Prerequisites

- Go 1.22+
- Ollama running on port 11434 with `gpt-oss:latest` pulled

## Run locally

```bash
go mod tidy
go run .
```

## Run with Docker

```bash
docker-compose up --build
```

## API

### Chat — parse a natural language message

```
POST /api/chat
{ "message": "spent 250 baht on lunch" }
```

Response:
```json
{
  "id": 1,
  "raw_message": "spent 250 baht on lunch",
  "type": "expense",
  "amount": 250,
  "category": "food",
  "description": "lunch",
  "created_at": "..."
}
```

### List all transactions

```
GET /api/transactions
```

### Delete a transaction

```
DELETE /api/transactions/:id
```

### Summary (balance)

```
GET /api/summary
```

Response:
```json
{
  "total_income": 50000,
  "total_expense": 1200,
  "balance": 48800
}
```

## Example messages

- `"got salary 50000 baht"`
- `"bought coffee 80 baht"`
- `"taxi to airport 350"`
- `"received freelance payment 5000"`
- `"dinner with friends 1200 baht"`

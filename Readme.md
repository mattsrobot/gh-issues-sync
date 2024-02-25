# Migrations

## Requirements

Have the following setup and running:

- Postgres, Redis, Meilisearch

## Database migrations

migrate -database "postgres://127.0.0.1:5432/issues?sslmode=disable" -path ./db/migrations/ up

## Run Database locally

postgres -D /opt/homebrew/var/postgres

## Creeate an .env file

Example: (fix as needed)

```
DATABASE_PRIVATE_URL="dbname=issues sslmode=disable"
REDIS_PRIVATE_URL="redis://localhost:6379/0"
MEILI_PRIVATE_URL="https://meilisearch-production.up.railway.app"
MEILI_API_KEY="your_api_key"
WS_API_PRIVATE_URL="http://localhost:5001/v1/internal"
```

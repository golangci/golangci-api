[![CircleCI](https://circleci.com/gh/golangci/golangci-api.svg?style=svg)](https://circleci.com/gh/golangci/golangci-api)
[![GolangCI](https://golangci.com/badges/github.com/golangci/golangci-api.svg)](https://golangci.com)

# API
This repository contains code of API.

## Development
### Technologies
Go (golang), heroku, circleci, docker, redis, postgres.

### Preparation
Run:
```
docker-compose up -d
echo "create database api_prod;" | docker-compose exec -T pg psql -Upostgres
```
It runs postgres and redis needed for both api and worker.

### How to run worker
```bash
make run_dev
```

### Configuration
Configurate via `.env` file. Dev `.env` may be like this:
```
WEB_ROOT="https://dev.golangci.com"
GITHUB_CALLBACK_HOST=https://api.dev.golangci.com
DATABASE_URL="postgresql://postgres:test@localhost:5432/api_prod?sslmode=disable"
REDIS_URL="redis://127.0.0.1:6379"
PORT=3000
```

Tests need `.env.test` file, overriding options from `.env`. There can be something like this:
```
DATABASE_URL="postgresql://postgres:test@localhost:5432/api_prod?sslmode=disable"
DATABASE_DEBUG=1
```

### How to run tests
```
echo "CREATE DATABASE api_prod;" | docker-compose exec -T pg psql -U postgres
make test
```

### How to test with web
Run golangci-web, golangci-worker and golangci-api. Go to `https://dev.golangci.com` locally and it will work.

# Contributing
See [CONTRIBUTING](https://github.com/golangci/golangci-api/blob/master/CONTRIBUTING.md).

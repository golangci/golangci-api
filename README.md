[![CircleCI](https://circleci.com/gh/golangci/golangci-api.svg?style=svg)](https://circleci.com/gh/golangci/golangci-api)
[![GolangCI](https://golangci.com/badges/github.com/golangci/golangci-api.svg)](https://golangci.com)

# API
This repository contains code of API.

## Development
### Technologies
Go (golang), heroku, circleci, docker, redis, postgres.
Web framework is a go kit wrapped with a [code generation](https://github.com/golangci/golangci-api/blob/master/cmd/genservices/main.go).

### Preparation
Run:
```
docker-compose up -d
echo "create database api_prod;" | docker-compose exec -T pg psql -Upostgres
```
It runs postgres and redis needed for both api and worker.

### How to run
```bash
make run_api
make run_worker
```

### Configuration
Configurate via `.env` file. Dev `.env` may be like this:
```
WEB_ROOT="https://dev.golangci.com"
API_URL="https://api.dev.golangci.com"
GITHUB_CALLBACK_HOST=https://api.dev.golangci.com
DATABASE_URL="postgresql://postgres:test@localhost:5432/api_prod?sslmode=disable"
REDIS_URL="redis://127.0.0.1:6379"
PORT=3000
APP_NAME="GolangCI Dev"
```

Tests need `.env.test` file, overriding options from `.env`. There can be something like this:
```
DATABASE_URL="postgresql://postgres:test@localhost:5432/api_test?sslmode=disable"
DATABASE_DEBUG=1
```

### How to run tests
```
echo "CREATE DATABASE api_test;" | docker-compose exec -T pg psql -U postgres
make test
```

### How to test with web
Run golangci-web, golangci-worker and golangci-api. Go to `https://dev.golangci.com` locally and it will work.

## Subscriptions and Payment Gateway

### Requirements

To use Subscriptions you will need to configure the env variables for the gateway of your choice.

* Note: Currently only SecurionPay is supported and uses `SECURIONPAY_SECRET` and `SECURIONPAY_PLANID`.

### Payment Gateway Callbacks

Run `ngrok http 3000` on your development machine, and use `https://{ngrok_id}.ngrok.io/v1/payment/{gateway}/events` as URL to receive events from the payment gateway.

* `{gateway}` for SecurionPay is `securionpay`.
* `{ngrok_id}`'s are unique and you must update the callback URL when you restart Ngrok service.

# Contributing
See [CONTRIBUTING](https://github.com/golangci/golangci-api/blob/master/CONTRIBUTING.md).

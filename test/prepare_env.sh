#!/bin/bash -x
echo 'REDIS_URL="redis://localhost:6379"' >>.env
echo 'WEB_ROOT="https://golangci.com"' >>.env
echo MIGRATIONS_PATH=../migrations >>.env
echo PATCH_STORE_DIR=/go >>.env
echo 'DATABASE_URL="postgresql://postgres:test@localhost:5432/api_test?sslmode=disable"' >>.env.test
echo 'SESSION_SECRET="123123123"' >> .env.test
echo GITHUB_KEY=GK >>.env.test
echo GITHUB_SECRET=GS >>.env.test
echo GITHUB_CALLBACK_HOST=GCH >>.env.test
echo SQS_PRIMARYDEADLETTER_QUEUE_URL=123 >>.env.test

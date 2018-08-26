build:
	go build -v ./app/cmd/...

gen:
	go generate ./...
	go run cmd/genservices/main.go

run_dev:
	godotenv go run cmd/golangci-api/main.go

migrate_example:
	godotenv -f .env.test sh -c 'migrate -database $${DATABASE_URL} -path ./app/migrations force 18'

test:
	go test -v -parallel 1 -p 1 ./app/handlers/...
	golangci-lint run -v


build:
	go build -v ./app/cmd/...

gen: gen_services
	go generate ./...

gen_services:
	go run cmd/genservices/main.go

run_env:
	SERVICES=sqs localstack start --docker

prepare_env:
	awslocal sqs create-queue --queue-name primary
	awslocal sqs list-queues

run_dev:
	godotenv go run cmd/golangci-api/main.go

migrate_force_version:
	godotenv -f .env.test sh -c 'migrate -database $${DATABASE_URL} -path ./migrations force $${V}'

test:
	go test -v -parallel 1 -p 1 ./app/test/
	golangci-lint run -v

connect_to_local_db:
	dc exec pg psql -U postgres -d api_prod

build_lambda:
	GOOS=linux go build -o sqsLambdaConsumer ./deployments/awslambda/sqsconsumer/
	zip sqsLambdaConsumer.zip sqsLambdaConsumer

deploy_lambda: build_lambda
	aws s3 cp ./sqsLambdaConsumer.zip s3://golangci-lambda-functions/

deploy_cloudformation:
	aws cloudformation deploy --template ./deployments/cloudformation.yml --region us-east-1 --capabilities CAPABILITY_IAM CAPABILITY_NAMED_IAM --stack-name golangci

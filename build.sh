#!/bin/bash

set -exu

# if [[ `git status --porcelain` ]]; then
#     echo has git changes;
#     echo exit 1;
# fi

if [ $1 = "api" ]; then
    GO111MODULE=on GOOS=linux CGO_ENABLED=0 GOARCH=amd64 \
        go build -o golangci-api-dlq-consumer \
        ./scripts/consume_dlq/main.go
fi

GO111MODULE=on GOOS=linux CGO_ENABLED=0 GOARCH=amd64 \
    go build \
    -ldflags "-s -w -X 'main.version=$(git name-rev --tags --name-only $(git rev-parse HEAD))' -X 'main.commit=$(git rev-parse --short HEAD)' -X 'main.date=$(date)'" \
    -o golangci-${1} \
    ./cmd/golangci-${1}/main.go
-include .env
export

BINARY := bin/api

.PHONY: run build test lint migrate-up migrate-down

run:
	go run ./cmd/api

build:
	go build -o $(BINARY) ./cmd/api

test:
	go test ./... -race -coverprofile=coverage.out

lint:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	go vet ./...

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

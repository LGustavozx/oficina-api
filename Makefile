.PHONY: run test cover up down build

build:
	go build -o bin/api ./cmd/api

run:
	go run ./cmd/api

test:
	go test ./... -race -cover

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

up:
	docker compose up --build

down:
	docker compose down

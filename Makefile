.PHONY: build run test clean docker-up docker-down migrate

# Development
build:
	go build -o bin/task-manager cmd/api/main.go

run:
	go run cmd/api/main.go

test:
	go test ./... -v

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

# Docker
docker-up:
	docker-compose up -d --build

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Database
migrate:
	go run cmd/migrate/main.go

# Code Quality
lint:
	golangci-lint run

format:
	go fmt ./...

tidy:
	go mod tidy

# All-in-one dev setup
dev: docker-up
	sleep 5
	@echo "Server starting..."
	go run cmd/api/main.go
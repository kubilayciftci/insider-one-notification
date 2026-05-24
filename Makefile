.PHONY: test test-unit test-integration lint mock-gen run-api run-worker docker-up docker-down migrate-up

test:
	go test ./... -v -count=1

test-unit:
	go test ./internal/core/... -v -count=1

test-integration:
	go test ./internal/adapters/... -v -count=1 -tags=integration

lint:
	golangci-lint run ./...

mock-gen:
	mockgen -source=internal/core/ports/repository.go -destination=internal/core/ports/mocks/repository_mock.go -package=mocks
	mockgen -source=internal/core/ports/queue.go -destination=internal/core/ports/mocks/queue_mock.go -package=mocks
	mockgen -source=internal/core/ports/notifier.go -destination=internal/core/ports/mocks/notifier_mock.go -package=mocks

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

docker-up:
	docker-compose up --build -d

docker-down:
	docker-compose down -v

migrate-up:
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$DATABASE_URL" down

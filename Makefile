# Проверяем существование .env файла и создаём из шаблона, если его нет
ifneq ("$(wildcard ./internal/config/.env)","")
include ./internal/config/.env
export
else
$(info Файл .env не найден. Создаём из шаблона...)
$(shell cp ./internal/config/.env.example ./internal/config/.env)
include ./internal/config/.env
export
$(info Создан файл .env из шаблона .env.example)
endif

.PHONY: all lint protoc run

all: protoc lint run
	@echo "Все шаги выполнены успешно!"

lint:
	@echo "Запуск линтера"
	golangci-lint run

protoc:
	@echo "Запуск protoc"
	protoc -I proto proto/telegram.proto --go_out=./proto --go_opt=paths=source_relative --go-grpc_out=./proto --go-grpc_opt=paths=source_relative

run:
	@echo "Запуск go run"
	go run cmd/main.go
.PHONY: build run clean test

build:
	docker compose build

run:
	docker compose up

clean:
	docker compose down -v

test:
	go test ./tests

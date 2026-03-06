.PHONY: build run test clean docker-up docker-down

build:
	go build -o bin/flowgate ./cmd/flowgate

run: build
	./bin/flowgate

test:
	go test ./... -v

clean:
	rm -rf bin/

docker-up:
	docker compose -f deployments/docker-compose.yml up --build

docker-down:
	docker compose -f deployments/docker-compose.yml down

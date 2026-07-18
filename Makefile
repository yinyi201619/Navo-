BINARY := navo-forum
PKG := navo-nt-forum
CMD_PATH := ./cmd/server
BUILD_DIR := bin

.PHONY: all build run tidy clean test migrate build-arm64 build-amd64 docker

all: build

tidy:
	go mod tidy

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY) $(CMD_PATH)

build-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY)-arm64 $(CMD_PATH)

build-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY)-amd64 $(CMD_PATH)

run:
	go run $(CMD_PATH) -config configs/config.yaml

migrate:
	go run $(CMD_PATH) -config configs/config.yaml -migrate

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)

docker:
	docker build -t navo-forum:latest .

docker-arm64:
	docker buildx build --platform linux/arm64 -t navo-forum:arm64 --load .

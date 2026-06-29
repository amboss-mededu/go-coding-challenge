run:
	go run ./cmd

build:
	go build -o bin/generator ./cmd

test:
	go clean -testcache && go test ./...

generate:
	./bin/generator

make deps:
	go mod tidy

build:
	go build -o bin/laburito ./cmd

build-railway:
	go build -ldflags="-w -s" -o out ./cmd

run: build
	./bin/laburito

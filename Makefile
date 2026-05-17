build:
	go build -o bin/laburito ./cmd

run: build
	./bin/laburito

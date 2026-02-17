.PHONY: build clean test run-api run-cli install

build:
	@echo "Building sdgen..."
	@mkdir -p bin
	go build -o bin/sdgen ./cmd/sdgen
	go build -o bin/sdgen-api ./cmd/sdgen-api
	@echo "Build complete: bin/sdgen, bin/sdgen-api"

clean:
	@echo "Cleaning..."
	rm -rf bin/
	@echo "Clean complete"

test:
	go test ./...

run-api: build
	./bin/sdgen-api

run-cli: build
	./bin/sdgen scenario list

install: build
	cp bin/sdgen $(GOPATH)/bin/
	cp bin/sdgen-api $(GOPATH)/bin/
	@echo "Installed to $(GOPATH)/bin/"

deps:
	go mod download
	go mod tidy

all: deps build

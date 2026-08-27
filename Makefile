.PHONY: all
all: build

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: build
build: tidy
	go build ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test ./...

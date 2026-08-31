.PHONY: build test vet tidy clean

BINARY := bin/henkaipan

build:
	@mkdir -p bin
	go build -o $(BINARY) ./cmd/henkaipan

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin dist
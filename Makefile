BINARY_NAME=gitai

all: build

build:
	go build -o bin/$(BINARY_NAME) main.go

install:
	go build -o bin/$(BINARY_NAME) main.go
	sudo mv bin/$(BINARY_NAME) /usr/local/bin/


# capture any extra make goals (positional) as the keyword string
ARGS := $(MAKECMDGOALS)
# grab second goal if present (first is the target name)
KEYWORDS_FROM_GOALS := $(word 2,$(ARGS))

# allow explicit KEYWORDS=... invocation to override positional
KEYWORDS := $(or $(KEYWORDS),$(KEYWORDS_FROM_GOALS))

# ignore unknown goals so an extra positional arg doesn't make make try to build a file
%:
	@:

test: 
	go test ./...

lint:
	golangci-lint run

clean:
	rm -f bin/$(BINARY_NAME)

deps:
	go mod tidy

fmt:
	go fmt ./...

run:
	go run main.go

.PHONY: all build test lint clean deps fmt run
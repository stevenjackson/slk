.PHONY: build build-slk build-slktui install clean test

build: build-slk build-slktui

build-slk:
	go build -o slk ./cmd/slk

build-slktui:
	go build -o slktui ./cmd/slktui

install:
	go install ./cmd/slk ./cmd/slktui

clean:
	rm -f slk slktui

test:
	go test ./...

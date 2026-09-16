# Vývojové ciele. Aplikácia je pre Windows, ale preložiť a otestovať sa dá
# aj na Linuxe či macOS – Go vie prekladať pre iný systém.

GOFLAGS := -trimpath
LDFLAGS := -H=windowsgui -s -w

.PHONY: all test vet build build-arm64 icons clean

all: test build

test:
	go test ./...
	GOOS=windows go vet ./...

vet:
	go vet ./...

build:
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/bateria.exe ./cmd/bateria

build-arm64:
	GOOS=windows GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/bateria-arm64.exe ./cmd/bateria

# Prekreslí assets/app.ico a docs/ikony.png zo zdrojového kódu ikony.
icons:
	go run ./cmd/icongen

clean:
	rm -rf dist

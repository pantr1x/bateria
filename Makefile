# Vývojové ciele. Aplikácia je pre Windows, ale preložiť a otestovať sa dá
# aj na Linuxe či macOS – Go vie prekladať pre iný systém.

GOFLAGS := -trimpath
LDFLAGS := -H=windowsgui -s -w

.PHONY: all test vet build build-arm64 icons hook clean

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

# Preloží bateria-hook.dll – knižnicu, ktorá vloží čas priamo do hodín
# v paneli úloh (vkladá sa do explorer.exe cez SetWindowsHookEx). Používateľ
# si ju prekladá sám podľa taskbarclock/README.md; toto je len skratka pre
# vývoj s nainštalovaným MinGW.
hook:
	x86_64-w64-mingw32-g++ -O2 -s -shared -static -municode \
		-o taskbarclock/bateria-hook.dll taskbarclock/hook.cpp -lkernel32 -luser32

clean:
	rm -rf dist

# Vývojové ciele. Aplikácia je pre Windows, ale preložiť a otestovať sa dá
# aj na Linuxe či macOS – Go vie prekladať pre iný systém.

GOFLAGS := -trimpath
LDFLAGS := -H=windowsgui -s -w

.PHONY: all test vet build build-arm64 icons panel clean

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

# Preloží program, ktorý kreslí text priamo v paneli úloh. Výsledok je
# zabalený v hlavnom programe, preto je v repozitári aj preložený – bez
# MinGW sa teda dá projekt zostaviť aj tak.
panel:
	x86_64-w64-mingw32-g++ -O2 -s -static -mwindows -municode -Wall \
		-o internal/panelbin/bateria-panel.exe panel/panel.cpp -lgdi32

clean:
	rm -rf dist

version := "dev"
commit := `git rev-parse --short HEAD 2>/dev/null || echo none`
date := `date -u +%Y-%m-%dT%H:%M:%SZ`
ldflags := "-s -w -X main.version={{version}} -X main.commit={{commit}} -X main.date={{date}}"

build:
    go build -ldflags '{{ldflags}}' -o oc-vpn-go ./cmd/oc-vpn

test:
    go test -count=1 ./...

clean:
    rm -f oc-vpn-go

deploy: build test
    ln -sf "$(pwd)/oc-vpn-go" /usr/local/bin/oc-vpn-go
    @echo "Done — $(oc-vpn-go version)"

# Bump version: just bump 1.0.0
bump ver:
    @echo "Tagging v{{ver}}..."
    git tag -a "v{{ver}}" -m "Release v{{ver}}"
    git push origin "v{{ver}}"
    @echo "v{{ver}} pushed — CI will build and release."

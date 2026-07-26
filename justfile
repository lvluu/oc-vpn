version := "dev"
commit := `git rev-parse --short HEAD 2>/dev/null || echo none`
date := `date -u +%Y-%m-%dT%H:%M:%SZ`
ldflags := "-s -w -X main.version={{version}} -X main.commit={{commit}} -X main.date={{date}}"

# ── Build ──────────────────────────────────────────────

build:
    go build -ldflags '{{ldflags}}' -o oc-vpn ./cmd/oc-vpn

clean:
    rm -f oc-vpn

# ── Quality ────────────────────────────────────────────

fmt:
    gofmt -l -w .
    goimports -w -local github.com/lvluu/oc-vpn .

lint:
    golangci-lint run ./...

vet:
    go vet ./...

test:
    go test -count=1 -race ./...

# ── All checks ─────────────────────────────────────────

check: lint vet test
    @echo "All checks passed."

# ── Deploy ─────────────────────────────────────────────

deploy: check build
    @mkdir -p "$(go env GOPATH)/bin"
    cp oc-vpn "$(go env GOPATH)/bin/oc-vpn-dev"
    @echo "Done — $(go env GOPATH)/bin/oc-vpn-dev"

# ── Release ────────────────────────────────────────────

# just bump 1.0.0
bump ver:
    @echo "Tagging v{{ver}}..."
    git tag -a "v{{ver}}" -m "Release v{{ver}}"
    git push origin "v{{ver}}"
    @echo "v{{ver}} pushed — CI will build and release."

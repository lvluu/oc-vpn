build:
    go build -o oc-vpn-go ./cmd/oc-vpn

test:
    go test ./...

deploy:
    go build -o oc-vpn-go ./cmd/oc-vpn
    go test ./...
    ln -sf "$(pwd)/oc-vpn-go" /usr/local/bin/oc-vpn-go
    @echo "Done — $(oc-vpn-go version)"

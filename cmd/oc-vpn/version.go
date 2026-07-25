package main

// Set via ldflags at build time:
//
//	go build -ldflags "-s -w -X main.version=v1.0.0 -X main.commit=abc123 -X main.date=2026-07-25"
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

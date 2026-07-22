package frontend

import "embed"

// Content contains the production build consumed by the Go web server.
//
//go:embed dist/*
var Content embed.FS

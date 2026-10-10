package frontend

import "embed"

// Content contains the production build consumed by the Go web server.
//
// Include underscore-prefixed bundler helpers required by lazy-loaded pages.
//
//go:embed all:dist/*
var Content embed.FS

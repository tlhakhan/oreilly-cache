// Package webui embeds the compiled frontend so the Go binary is self-contained.
package webui

import "embed"

// FS holds the contents of web/dist/ as built by `make web-build`.
// The dist/ subdirectory must be stripped before serving — use fs.Sub(FS, "dist").
//
//go:embed all:dist
var FS embed.FS

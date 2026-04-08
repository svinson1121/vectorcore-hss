// Package webui exposes the compiled React UI as an embedded filesystem.
// The embed path "dist" is relative to this file (web/dist/).
// Build the UI with `make ui` before compiling the binary.
package webui

import "embed"

//go:embed all:dist
var FS embed.FS

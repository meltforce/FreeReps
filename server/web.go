package freereps

import "embed"

// WebFS contains the embedded frontend build output.
//
//go:embed all:web/dist
var WebFS embed.FS

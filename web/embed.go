// Package web holds the built Vue frontend, embedded so hetud ships self-contained.
package web

import "embed"

//go:embed all:dist
var Dist embed.FS

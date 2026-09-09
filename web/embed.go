// Package web embeds the browser chart so cmd/serve is a single
// binary. go:embed cannot walk out of a package directory, so the
// files live here rather than next to cmd/serve.
package web

import "embed"

//go:embed index.html chart.js replay.js footprint.js profile.js timesales.js heatmap.js
var FS embed.FS

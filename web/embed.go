// Package webdist embeds the production Vite build output (`web/dist`).
//
// This Go file lives at the same level as the `dist/` directory because
// `//go:embed` patterns are relative to the source file. The rest of
// `web/` (package.json, src/, vite.config.ts, ...) is invisible to the
// Go toolchain — Go only sees this single file.
//
// A committed `web/dist/.keep` placeholder lets the embed succeed before
// `npm run build` has populated the directory.
package webdist

import "embed"

//go:embed all:dist
var Assets embed.FS

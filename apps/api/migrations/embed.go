// Package migrations holds the ordered SQL files the API applies on startup.
package migrations

import "embed"

// FS contains every numbered migration.
//
//go:embed *.sql
var FS embed.FS

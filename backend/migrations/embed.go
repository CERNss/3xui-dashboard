// Package migrations holds the embedded versioned SQL files applied
// by the repository.MigrateUp runner at startup. Adding a new
// migration is a two-file change (NNNN_<topic>.up.sql + .down.sql);
// the embed directive picks them up automatically.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

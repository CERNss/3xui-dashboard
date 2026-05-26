// Package migrations holds the embedded SQL files applied by the
// repository.MigrateUp runner at startup. The current pre-launch schema is
// collapsed into 0001_init; future deployed schema changes can add a new
// NNNN_<topic>.up.sql + .down.sql pair.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

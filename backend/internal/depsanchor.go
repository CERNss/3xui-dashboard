//go:build depsanchor

// File depsanchor.go anchors module versions for runtime dependencies
// that will be imported by feature code in later task groups (DB,
// migrations, JWT, cron jobs, bcrypt, errgroup). The blank imports
// keep `go mod tidy` from stripping these modules before the feature
// code lands. The `depsanchor` build tag means this file is not part
// of any production build — it exists purely to influence go.mod.
//
// Delete this file once every dep listed below has a real importer
// in the codebase.

package internal

import (
	_ "github.com/golang-jwt/jwt/v5"
	_ "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/robfig/cron/v3"
	_ "golang.org/x/crypto/bcrypt"
	_ "golang.org/x/sync/errgroup"
	_ "gorm.io/driver/postgres"
	_ "gorm.io/gorm"
)

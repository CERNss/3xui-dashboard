//go:build depsanchor

// File depsanchor.go anchors module versions for runtime dependencies
// that will be imported by feature code in later task groups (JWT,
// cron jobs, bcrypt, errgroup). The blank imports keep `go mod tidy`
// from stripping these modules before the feature code lands. The
// `depsanchor` build tag means this file is not part of any production
// build — it exists purely to influence go.mod.
//
// Delete entries as their owning packages start to import them
// directly; delete the whole file once everything has a real importer.

package internal

import (
	_ "github.com/golang-jwt/jwt/v5"
	_ "github.com/robfig/cron/v3"
	_ "golang.org/x/crypto/bcrypt"
	_ "golang.org/x/sync/errgroup"
)

package repository

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/migrations"
)

// MigrateUp runs every pending migration against db using
// golang-migrate's iofs source. Returns nil — and logs a "schema up to
// date" line — when the database is already at the latest version.
func MigrateUp(db *gorm.DB, lg *slog.Logger) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("migrate: extract *sql.DB: %w", err)
	}

	driver, err := migratepostgres.WithInstance(sqlDB, &migratepostgres.Config{})
	if err != nil {
		return fmt.Errorf("migrate: build driver: %w", err)
	}

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("migrate: build source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate: assemble migrator: %w", err)
	}
	m.Log = migrateLogAdapter{lg: lg.With(slog.String("component", "migrate"))}

	beforeVer, _, _ := m.Version()
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			lg.Info("database schema up to date",
				slog.Uint64("version", uint64(beforeVer)),
			)
			return nil
		}
		return fmt.Errorf("migrate up: %w", err)
	}

	afterVer, _, _ := m.Version()
	lg.Info("database schema migrated",
		slog.Uint64("from", uint64(beforeVer)),
		slog.Uint64("to", uint64(afterVer)),
	)
	return nil
}

type migrateLogAdapter struct{ lg *slog.Logger }

func (a migrateLogAdapter) Printf(format string, v ...any) {
	a.lg.Info(fmt.Sprintf(format, v...))
}

func (migrateLogAdapter) Verbose() bool { return false }

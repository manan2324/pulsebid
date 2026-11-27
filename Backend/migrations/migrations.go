// Package migrations owns the embedded, versioned database schema.
package migrations

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// files contains read-only embedded migration assets.
//
//go:embed *.sql
var files embed.FS

// Apply applies all pending migrations, or reverses one version for down.
// PostgreSQL statement/lock timeouts bound calls inside golang-migrate, whose
// database driver does not accept a caller context on Up/Steps.
func Apply(ctx context.Context, dsn, direction string) (result error) {
	if direction != "up" && direction != "down" {
		return fmt.Errorf("migration direction must be up or down")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("invalid DATABASE_URL")
	}
	cfg.ConnectTimeout = 5 * time.Second
	cfg.RuntimeParams["statement_timeout"] = "30000"
	cfg.RuntimeParams["lock_timeout"] = "5000"
	connection := stdlib.OpenDB(*cfg)
	defer func() { result = errors.Join(result, connection.Close()) }()
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = connection.PingContext(pingCtx); err != nil {
		return fmt.Errorf("migration database unavailable: %w", err)
	}
	driver, err := migratepgx.WithInstance(connection, &migratepgx.Config{DatabaseName: cfg.Database, SchemaName: "public", StatementTimeout: 30 * time.Second})
	if err != nil {
		return fmt.Errorf("initialize migration driver: %w", err)
	}
	source, err := iofs.New(files, ".")
	if err != nil {
		return err
	}
	runner, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		return err
	}
	defer func() { a, b := runner.Close(); result = errors.Join(result, a, b) }()
	runner.LockTimeout = 5 * time.Second
	if direction == "up" {
		err = runner.Up()
	} else {
		err = runner.Steps(-1)
	}
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

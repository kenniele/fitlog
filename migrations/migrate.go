package migrations

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// migrationFiles keeps production migrations inside the application image so
// deploys do not depend on Go, goose, or a checkout-mounted directory on the
// server.
//
//go:embed *.sql
var migrationFiles embed.FS

// Up applies all pending SQL migrations to the PostgreSQL database.
func Up(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	goose.SetBaseFS(migrationFiles)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set migration dialect: %w", err)
	}
	if err := baselineLegacySchema(ctx, db); err != nil {
		return fmt.Errorf("baseline legacy schema: %w", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}

// baselineLegacySchema records migrations that predate automated deployment.
// Older Fitlog databases already contain oauth_tokens and notes, but have no
// goose history, so applying migration 00001 again would fail. Fresh databases
// are left untouched and are migrated normally by goose.
func baselineLegacySchema(ctx context.Context, db *sql.DB) error {
	var versionTableExists, oauthTokensExists, notesExists bool
	if err := db.QueryRowContext(ctx, `
		SELECT
			to_regclass('public.goose_db_version') IS NOT NULL,
			to_regclass('public.oauth_tokens') IS NOT NULL,
			to_regclass('public.notes') IS NOT NULL
	`).Scan(&versionTableExists, &oauthTokensExists, &notesExists); err != nil {
		return fmt.Errorf("inspect schema: %w", err)
	}

	legacyVersion := int64(0)
	if oauthTokensExists {
		legacyVersion = 1
	}
	if notesExists {
		if !oauthTokensExists {
			return errors.New("notes exists but oauth_tokens is missing")
		}
		legacyVersion = 2
	}
	if legacyVersion == 0 {
		return nil
	}

	if !versionTableExists {
		if _, err := goose.EnsureDBVersionContext(ctx, db); err != nil {
			return fmt.Errorf("create version table: %w", err)
		}
	}

	var currentVersion int64
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_id) FILTER (WHERE is_applied), 0)
		FROM goose_db_version
	`).Scan(&currentVersion); err != nil {
		return fmt.Errorf("read current version: %w", err)
	}
	if currentVersion >= legacyVersion {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin baseline transaction: %w", err)
	}
	defer tx.Rollback()

	for version := currentVersion + 1; version <= legacyVersion; version++ {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO goose_db_version (version_id, is_applied)
			VALUES ($1, true)
		`, version); err != nil {
			return fmt.Errorf("record version %d: %w", version, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit baseline: %w", err)
	}

	return nil
}

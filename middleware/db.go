// Copyright 2025 Neomantra Corp

package middleware

import (
	"bytes"
	"database/sql"
	_ "embed" // Required for go:embed
	"fmt"
	"html/template"
)

// MigrationInfo holds data to be injected by our migration template
type MigrationInfo struct {
	MigrationName string
	TableName     string
}

///////////////////////////////////////////////////////////////////////////////

// ExtenstionsMigrationTempl is the SQL format string for installing extensions
//
//go:embed sql/extensions.sql.tpl
var ExtensionsMigrationTemplate string

// TradeMigrationTempl is the SQL format string for trades table migration
// Takes the "TableName"
//
//go:embed sql/trades.sql.tpl
var TradeMigrationTemplate string

// candlesMigrationTempl is the SQL format string for candles table migration
// Takes the "TableName"
//
//go:embed sql/candles.sql.tpl
var CandlesMigrationTemplate string

///////////////////////////////////////////////////////////////////////////////

// RunMigration executes the templated migration string on the DuckDB connection.
// Returns an error, if any.
func RunMigration(duckdbConn *sql.DB, migrationTemplate string, info MigrationInfo) error {
	migrationTempl, err := template.New(info.MigrationName).Parse(migrationTemplate)
	if err != nil {
		return fmt.Errorf("failed to create template migration: %w", err)
	}
	var migrationBytes bytes.Buffer
	if err = migrationTempl.Execute(&migrationBytes, info); err != nil {
		return fmt.Errorf("failed to template migration: %w", err)
	}

	_, err = duckdbConn.Exec(migrationBytes.String())
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}
	return nil
}

package postgres

import (
	"database/sql"
	"fmt"
	"strconv"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// PostgresDriver encapsulates the database connection.
type PostgresDriver struct {
	db *sql.DB
}

// New initializes the database connection and ensures the migrations table exists.
func New(url string) (*PostgresDriver, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	// Create schema_migrations table if it doesn't exist.
	// Using a composite primary key on (version_name, direction) is a good choice
	// as it naturally prevents running the same migration/direction pair twice.
	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS schema_migrations (
      version_name TEXT NOT NULL,
      applied_at TIMESTAMP WITH TIME ZONE NOT NULL,
      dirty BOOLEAN NOT NULL,
      direction TEXT NOT NULL CHECK (direction IN ('up','down')),
      PRIMARY KEY (version_name, direction)
    );`)
	if err != nil {
		return nil, fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	return &PostgresDriver{db: db}, nil
}

// DB returns the underlying *sql.DB object.
func (p *PostgresDriver) DB() *sql.DB {
	return p.db
}

// Run executes a SQL statement within a transaction.
func (p *PostgresDriver) Run(sqlStmt string) error {
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback is a no-op if Commit succeeds.

	if _, err = tx.Exec(sqlStmt); err != nil {
		return fmt.Errorf("failed to execute sql statement: %w", err)
	}

	return tx.Commit()
}

// GetApplied returns a map of successfully applied migration version names for a given direction.
func (p *PostgresDriver) GetApplied(direction string) (map[string]bool, error) {
	rows, err := p.db.Query(`SELECT version_name FROM schema_migrations WHERE direction=$1 AND dirty=false`, direction)
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var versionName string
		if err := rows.Scan(&versionName); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}
		applied[versionName] = true
	}
	return applied, rows.Err()
}

// SetMigrationState uses an "upsert" operation to create or update a migration's state.
// This is more atomic and efficient than the previous UPDATE-then-INSERT logic.
func (p *PostgresDriver) SetMigrationState(versionName string, dirty bool, direction string) error {
	_, err := p.db.Exec(`
        INSERT INTO schema_migrations (version_name, dirty, direction, applied_at)
        VALUES ($1, $2, $3, NOW())
        ON CONFLICT (version_name, direction) DO UPDATE
        SET dirty = EXCLUDED.dirty, applied_at = EXCLUDED.applied_at;
    `, versionName, dirty, direction)

	if err != nil {
		return fmt.Errorf("failed to set migration state for %s: %w", versionName, err)
	}
	return nil
}

// Version returns the latest successfully applied migration version number.
// The logic is corrected to properly parse the numeric prefix from the version name.
func (p *PostgresDriver) Version() (int, bool, error) {
	var versionName string
	var dirty bool

	err := p.db.QueryRow(`
        SELECT version_name, dirty FROM schema_migrations
        WHERE direction='up'
        ORDER BY version_name DESC LIMIT 1
    `).Scan(&versionName, &dirty)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil // No migrations applied yet
		}
		return 0, false, fmt.Errorf("failed to query for current version: %w", err)
	}

	if len(versionName) < 6 {
		return 0, dirty, fmt.Errorf("invalid version name format found: %s", versionName)
	}

	version, err := strconv.Atoi(versionName[:6])
	if err != nil {
		return 0, dirty, fmt.Errorf("failed to parse version number from '%s': %w", versionName, err)
	}

	return version, dirty, nil
}

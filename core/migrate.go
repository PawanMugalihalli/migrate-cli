package core

import (
	"database/sql"
	"fmt"
	"migrate/gomigration"
	"path/filepath"
	"sort"
	"time"
)

// Interfaces define the contracts for migration sources and database drivers.
type Source interface {
	ReadUp(version int) (string, error)
	ReadDown(version int) (string, error)
	ListMigrations() ([]MigrationMetadata, error)
}

type Database interface {
	Version() (int, bool, error)
	SetMigrationState(versionName string, dirty bool, direction string) error
	GetApplied(direction string) (map[string]bool, error)
	Run(sql string) error
	DB() *sql.DB
}

// MigrationMetadata now explicitly holds the up and down filenames.
type MigrationMetadata struct {
	Version      int
	Name         string
	UpFile       string // e.g., "000001_create_users.up.sql"
	DownFile     string // e.g., "000001_create_users.down.sql"
	Dependencies []int
}

// Migrator orchestrates the migration process.
type Migrator struct {
	src Source
	db  Database
}

func New(src Source, db Database) *Migrator {
	return &Migrator{src: src, db: db}
}

// Up applies all pending migrations in the correct topological order.
func (m *Migrator) Up() error {
	migrations, err := m.getPendingMigrations()
	if err != nil {
		return err
	}
	if len(migrations) == 0 {
		fmt.Println("Database is already up to date.")
		return nil
	}
	fmt.Printf("Found %d pending migrations to apply.\n", len(migrations))
	return m.applyMigrations(migrations, "up")
}

// Down rolls back the single most recent 'up' migration.
// FIXED: This function is now updated to work with the new metadata struct.
func (m *Migrator) Down() error {
	rows, err := m.db.DB().Query(`
        SELECT version_name FROM schema_migrations
        WHERE direction='up' AND dirty=false
        ORDER BY version_name DESC LIMIT 1`)
	if err != nil {
		return fmt.Errorf("failed to find last migration: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		fmt.Println("No applied migrations to roll back.")
		return nil
	}

	var versionNameToRollback string
	if err := rows.Scan(&versionNameToRollback); err != nil {
		return err
	}

	allMigs, err := m.src.ListMigrations()
	if err != nil {
		return fmt.Errorf("could not list migration files: %w", err)
	}

	var migrationToRollback MigrationMetadata
	found := false
	for _, mig := range allMigs {
		if mig.UpFile == versionNameToRollback {
			migrationToRollback = mig
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("migration file for applied version %s not found", versionNameToRollback)
	}

	fmt.Printf("Rolling back migration: %s\n", migrationToRollback.DownFile)

	if err := m.db.SetMigrationState(migrationToRollback.DownFile, true, "down"); err != nil {
		return err
	}

	if err := m.runMigration(migrationToRollback, "down"); err != nil {
		return fmt.Errorf("failed to run down migration for %s: %w", migrationToRollback.DownFile, err)
	}

	if err := m.db.SetMigrationState(migrationToRollback.DownFile, false, "down"); err != nil {
		return err
	}

	_, err = m.db.DB().Exec(`DELETE FROM schema_migrations WHERE version_name=$1 AND direction='up'`, versionNameToRollback)
	if err == nil {
		fmt.Printf("Successfully rolled back %s\n", versionNameToRollback)
	}
	return err
}

// Goto applies all migrations up to and including the target version.
// FIXED: This function is now updated to work with the new metadata struct.
func (m *Migrator) Goto(targetVersion int) error {
	allMigs, err := m.src.ListMigrations()
	if err != nil {
		return err
	}

	targetDepGraph, err := m.buildDependencyGraph(allMigs, targetVersion)
	if err != nil {
		return err
	}

	sorted, err := TopoSort(targetDepGraph)
	if err != nil {
		return err
	}

	applied, err := m.db.GetApplied("up")
	if err != nil {
		return err
	}

	pending := make([]MigrationMetadata, 0)
	for _, mig := range sorted {
		if !applied[mig.UpFile] { // Simplified check
			pending = append(pending, mig)
		}
	}

	if len(pending) == 0 {
		fmt.Printf("All migrations up to version %d are already applied.\n", targetVersion)
		return nil
	}

	fmt.Printf("Applying %d migrations to reach version %d...\n", len(pending), targetVersion)
	return m.applyMigrations(pending, "up")
}

// Status prints the state of all migrations recorded in the database.
func (m *Migrator) Status() error {
	rows, err := m.db.DB().Query(`SELECT version_name, direction, applied_at, dirty FROM schema_migrations ORDER BY applied_at ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Println("Applied Migrations Status:")
	fmt.Println("---------------------------------------------------------------------")
	fmt.Printf("%-40s | %-10s | %-25s | %s\n", "VERSION NAME", "DIRECTION", "APPLIED AT", "DIRTY")
	fmt.Println("---------------------------------------------------------------------")
	for rows.Next() {
		var version, direction string
		var appliedAt time.Time
		var dirty bool
		if err := rows.Scan(&version, &direction, &appliedAt, &dirty); err != nil {
			return err
		}
		fmt.Printf("%-40s | %-10s | %-25s | %v\n", version, direction, appliedAt.Format(time.RFC3339), dirty)
	}
	fmt.Println("---------------------------------------------------------------------")
	return rows.Err()
}

// --- Helper Functions ---

// applyMigrations is a generic helper to run a list of migrations.
// FIXED: This function is now updated to call the new runMigration correctly.
func (m *Migrator) applyMigrations(migrations []MigrationMetadata, direction string) error {
	for _, mig := range migrations {
		filename := mig.UpFile
		if direction == "down" {
			filename = mig.DownFile
		}

		fmt.Printf("Applying: %s\n", filename)
		if err := m.db.SetMigrationState(filename, true, direction); err != nil {
			return err
		}

		if err := m.runMigration(mig, direction); err != nil {
			return fmt.Errorf("migration %s failed: %w", filename, err)
		}

		if err := m.db.SetMigrationState(filename, false, direction); err != nil {
			return err
		}
	}
	fmt.Println("All migrations applied successfully.")
	return nil
}

// runMigration executes a single migration file (.sql or .go).
// FIXED: The signature is updated and the SQL logic is filled in.
func (m *Migrator) runMigration(mig MigrationMetadata, direction string) error {
	var filename string
	if direction == "up" {
		filename = mig.UpFile
	} else {
		filename = mig.DownFile
	}

	if filename == "" {
		return fmt.Errorf("no '%s' migration file found for version %d", direction, mig.Version)
	}

	fileType := filepath.Ext(filename)

	switch fileType {
	case ".sql":
		var sqlStmt string
		var err error
		if direction == "up" {
			sqlStmt, err = m.src.ReadUp(mig.Version)
		} else {
			sqlStmt, err = m.src.ReadDown(mig.Version)
		}
		if err != nil {
			return err
		}
		//fmt.Printf("Executing SQL migration: %s\n", filename)
		return m.db.Run(sqlStmt)

	case ".go":
		goMig, exists := gomigration.GoMigrationRegistry[filename]
		if !exists {
			return fmt.Errorf("go migration %s is not registered", filename)
		}
		//fmt.Printf("Executing Go migration: %s\n", filename)
		return goMig.Run(m.db.DB())

	default:
		return fmt.Errorf("unsupported migration type: %s", fileType)
	}
}

// getPendingMigrations lists all migrations and filters out those already applied.
// FIXED: This function now correctly checks for applied migrations using the UpFile field.
func (m *Migrator) getPendingMigrations() ([]MigrationMetadata, error) {
	allMigs, err := m.src.ListMigrations()
	if err != nil {
		return nil, err
	}

	sorted, err := TopoSort(allMigs)
	if err != nil {
		return nil, err
	}

	applied, err := m.db.GetApplied("up")
	if err != nil {
		return nil, err
	}

	pending := make([]MigrationMetadata, 0)
	for _, mig := range sorted {
		if !applied[mig.UpFile] {
			pending = append(pending, mig)
		}
	}
	return pending, nil
}

// Inside core/migrator.go

// buildDependencyGraph creates a sub-graph containing only the target and its dependencies.
func (m *Migrator) buildDependencyGraph(allMigs []MigrationMetadata, targetVersion int) ([]MigrationMetadata, error) {
	// Step 1: Create a map for quick lookup of migrations by their version number.
	migMap := make(map[int]MigrationMetadata)
	for _, m := range allMigs {
		migMap[m.Version] = m
	}

	// Ensure the target migration actually exists.
	if _, exists := migMap[targetVersion]; !exists {
		return nil, fmt.Errorf("target migration version %d not found", targetVersion)
	}

	// Step 2: Use a map to track all collected dependency versions to avoid duplicates.
	depGraph := make(map[int]bool)

	// Step 3: Define a recursive function (a closure) to perform a Depth-First Search (DFS).
	var collectDeps func(int) error
	collectDeps = func(v int) error {
		// If we've already processed this migration, stop.
		if depGraph[v] {
			return nil
		}

		mig, ok := migMap[v]
		if !ok {
			return fmt.Errorf("dependency migration %d not found", v)
		}

		// Mark this migration as part of the dependency graph.
		depGraph[v] = true

		// Recursively call this function for all of the current migration's dependencies.
		for _, dep := range mig.Dependencies {
			if err := collectDeps(dep); err != nil {
				return err
			}
		}
		return nil
	}

	// Step 4: Start the recursive search from the target version.
	if err := collectDeps(targetVersion); err != nil {
		return nil, err
	}

	// Step 5: Build the final list of migration metadata.
	// Iterate through the original list and include only the ones we collected.
	result := make([]MigrationMetadata, 0, len(depGraph))
	for _, mig := range allMigs {
		if depGraph[mig.Version] {
			result = append(result, mig)
		}
	}
	return result, nil
}

func TopoSort(migrations []MigrationMetadata) ([]MigrationMetadata, error) {
	graph := make(map[int][]int)
	migMap := make(map[int]MigrationMetadata)
	for _, m := range migrations {
		graph[m.Version] = m.Dependencies
		migMap[m.Version] = m
	}

	var sorted []MigrationMetadata
	visited := make(map[int]bool)
	tempMarked := make(map[int]bool)

	var visit func(int) error
	visit = func(v int) error {
		if tempMarked[v] {
			return fmt.Errorf("cycle detected in migration dependencies at version %d", v)
		}
		if visited[v] {
			return nil
		}
		tempMarked[v] = true
		visited[v] = true

		for _, dep := range graph[v] {
			if _, ok := migMap[dep]; !ok {
				return fmt.Errorf("migration %d has an unknown dependency: %d", v, dep)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		tempMarked[v] = false
		sorted = append(sorted, migMap[v])
		return nil
	}

	sortedKeys := make([]int, 0, len(migrations))
	for _, m := range migrations {
		sortedKeys = append(sortedKeys, m.Version)
	}
	sort.Ints(sortedKeys)

	for _, v := range sortedKeys {
		if !visited[v] {
			if err := visit(v); err != nil {
				return nil, err
			}
		}
	}
	return sorted, nil
}

package core

import (
	"database/sql"
	"fmt"
	"migrate/gomigration"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Interfaces remain the same.
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

// MigrationMetadata remains the same.
type MigrationMetadata struct {
	Version      int
	Name         string
	UpFile       string
	DownFile     string
	Dependencies []int
}

// Migrator struct with caches remains the same.
type Migrator struct {
	src Source
	db  Database

	allMigrations          []MigrationMetadata
	allMigrationsErr       error
	fetchAllMigrationsOnce sync.Once

	appliedUpMigrations    map[string]bool
	appliedUpMigrationsErr error
	fetchAppliedUpOnce     sync.Once
}

func New(src Source, db Database) *Migrator {
	return &Migrator{src: src, db: db}
}

// --- Caching Helper Methods (Unchanged) ---
func (m *Migrator) getAllMigrations() ([]MigrationMetadata, error) {
	m.fetchAllMigrationsOnce.Do(func() {
		m.allMigrations, m.allMigrationsErr = m.src.ListMigrations()
	})
	return m.allMigrations, m.allMigrationsErr
}

func (m *Migrator) getAppliedUpMigrations() (map[string]bool, error) {
	m.fetchAppliedUpOnce.Do(func() {
		m.appliedUpMigrations, m.appliedUpMigrationsErr = m.db.GetApplied("up")
	})
	return m.appliedUpMigrations, m.appliedUpMigrationsErr
}

// --- Main Methods ---

// Goto is now fully implemented to handle both up and down migrations.
func (m *Migrator) Goto(targetVersion int) error {
	currentVersion, _, err := m.db.Version()
	if err != nil {
		return fmt.Errorf("failed to get current database version: %w", err)
	}

	if targetVersion == currentVersion {
		fmt.Printf("Database is already at version %d.\n", targetVersion)
		return nil
	}

	if targetVersion > currentVersion {
		return m.migrateUpTo(targetVersion)
	}

	return m.migrateDownTo(targetVersion)
}

// migrateUpTo contains the logic for migrating UP to a target.
func (m *Migrator) migrateUpTo(targetVersion int) error {
	allMigs, err := m.getAllMigrations()
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

	applied, err := m.getAppliedUpMigrations()
	if err != nil {
		return err
	}

	pending := make([]MigrationMetadata, 0)
	for _, mig := range sorted {
		if !applied[mig.UpFile] {
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

// migrateDownTo contains the logic for migrating DOWN to a target (rolling back).
func (m *Migrator) migrateDownTo(targetVersion int) error {
	allMigs, err := m.getAllMigrations()
	if err != nil {
		return err
	}

	// 1. Get the dependency graph for the target state.
	targetDepGraph, err := m.buildDependencyGraph(allMigs, targetVersion)
	if err != nil {
		return err
	}
	targetDepMap := make(map[int]bool)
	for _, mig := range targetDepGraph {
		targetDepMap[mig.Version] = true
	}

	// 2. Get the full list of currently applied migrations.
	applied, err := m.getAppliedUpMigrations()
	if err != nil {
		return err
	}
	appliedMigs := make([]MigrationMetadata, 0)
	for _, mig := range allMigs {
		if applied[mig.UpFile] {
			appliedMigs = append(appliedMigs, mig)
		}
	}

	// 3. Topologically sort the *currently applied* migrations to get the exact application order.
	appliedOrder, err := TopoSort(appliedMigs)
	if err != nil {
		return fmt.Errorf("could not sort applied migrations: %w", err)
	}

	// 4. Find which migrations to roll back by filtering the correctly sorted list.
	toRollback := make([]MigrationMetadata, 0)
	for _, mig := range appliedOrder {
		if !targetDepMap[mig.Version] {
			toRollback = append(toRollback, mig)
		}
	}

	if len(toRollback) == 0 {
		fmt.Println("No migrations to roll back.")
		return nil
	}

	// 5. IMPORTANT: Reverse the list to get the correct rollback order.
	// The last one applied must be the first one rolled back.
	for i, j := 0, len(toRollback)-1; i < j; i, j = i+1, j-1 {
		toRollback[i], toRollback[j] = toRollback[j], toRollback[i]
	}

	fmt.Printf("Rolling back %d migrations to reach version %d...\n", len(toRollback), targetVersion)
	// Note: We use a custom rollback loop instead of applyMigrations
	// because the state management is slightly different.
	for _, mig := range toRollback {
		fmt.Printf("Rolling back: %s\n", mig.DownFile)

		// Standard dirty flag management for the 'down' migration
		if err := m.db.SetMigrationState(mig.DownFile, true, "down"); err != nil {
			return err
		}
		if err := m.runMigration(mig, "down"); err != nil {
			return fmt.Errorf("failed to run down migration for %s: %w", mig.DownFile, err)
		}
		if err := m.db.SetMigrationState(mig.DownFile, false, "down"); err != nil {
			return err
		}

		// Clean up the original 'up' migration record
		_, err := m.db.DB().Exec(`DELETE FROM schema_migrations WHERE version_name=$1 AND direction='up'`, mig.UpFile)
		if err != nil {
			return fmt.Errorf("failed to delete 'up' record for %s: %w", mig.UpFile, err)
		}
	}

	fmt.Println("Rollback completed successfully.")
	return nil
}

// --- Other methods (Unchanged) ---

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

	allMigs, err := m.getAllMigrations()
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

func (m *Migrator) getPendingMigrations() ([]MigrationMetadata, error) {
	allMigs, err := m.getAllMigrations()
	if err != nil {
		return nil, err
	}

	sorted, err := TopoSort(allMigs)
	if err != nil {
		return nil, err
	}

	applied, err := m.getAppliedUpMigrations()
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
		return m.db.Run(sqlStmt)

	case ".go":
		goMig, exists := gomigration.GoMigrationRegistry[filename]
		if !exists {
			return fmt.Errorf("go migration %s is not registered", filename)
		}
		return goMig.Run(m.db.DB())

	default:
		return fmt.Errorf("unsupported migration type: %s", fileType)
	}
}

func (m *Migrator) buildDependencyGraph(allMigs []MigrationMetadata, targetVersion int) ([]MigrationMetadata, error) {
	migMap := make(map[int]MigrationMetadata)
	for _, m := range allMigs {
		migMap[m.Version] = m
	}
	if _, exists := migMap[targetVersion]; !exists {
		// Allow target 0 for rolling back all migrations
		if targetVersion == 0 {
			return []MigrationMetadata{}, nil
		}
		return nil, fmt.Errorf("target migration version %d not found", targetVersion)
	}
	depGraph := make(map[int]bool)
	var collectDeps func(int) error
	collectDeps = func(v int) error {
		if depGraph[v] {
			return nil
		}
		mig, ok := migMap[v]
		if !ok {
			return fmt.Errorf("dependency migration %d not found", v)
		}
		depGraph[v] = true
		for _, dep := range mig.Dependencies {
			if err := collectDeps(dep); err != nil {
				return err
			}
		}
		return nil
	}
	if err := collectDeps(targetVersion); err != nil {
		return nil, err
	}
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

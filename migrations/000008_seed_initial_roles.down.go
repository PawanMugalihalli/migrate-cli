package migrations

import (
	"database/sql"
	"log"

	"github.com/PawanMugalihalli/migrate-cli/gomigration" // <-- Make sure this matches your go.mod module name
)

func init() {
	gomigration.GoMigrationRegistry["000008_seed_initial_roles.down.go"] = &SeedInitialRolesDown{}
}

type SeedInitialRolesDown struct{}

func (s *SeedInitialRolesDown) Run(db *sql.DB) error {
	log.Println("Rolling back Go migration: removing initial roles...")
	_, err := db.Exec(`DELETE FROM auth.roles WHERE name IN ('Admin', 'Member');`)
	return err
}
func (s *SeedInitialRolesDown) Dependencies() []int {
	// Depends on 000003_create_roles_table.up.sql
	return []int{}
}

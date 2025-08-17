// migrations/000008_seed_initial_roles.up.go
package migrations

import (
	"database/sql"
	"log"
	"migrate/gomigration"
	"migrate/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func init() {
	gomigration.GoMigrationRegistry["000008_seed_initial_roles.up.go"] = &SeedInitialRolesUp{}
}

type SeedInitialRolesUp struct{}

func (s *SeedInitialRolesUp) Run(db *sql.DB) error {
	log.Println("Running GORM migration: seeding initial roles...")

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}

	rolesToSeed := []models.Role{
		{Name: "Admin"},
		{Name: "Member"},
	}

	for _, role := range rolesToSeed {
		result := gormDB.FirstOrCreate(&role, models.Role{Name: role.Name})
		if result.Error != nil {
			return result.Error
		}
	}

	return nil
}

// Dependencies specifies that this migration depends on the 'create_roles_table' migration.
func (s *SeedInitialRolesUp) Dependencies() []int {
	// Depends on 000003_create_roles_table.up.sql
	return []int{3}
}

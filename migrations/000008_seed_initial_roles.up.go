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

// The Role struct is no longer defined here.

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

	// Use the imported models.Role struct
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

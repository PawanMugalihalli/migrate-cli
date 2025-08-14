package gomigration

import "database/sql"

type Migration interface {
	Run(db *sql.DB) error
}

var GoMigrationRegistry = make(map[string]Migration)

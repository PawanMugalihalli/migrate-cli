package gomigration

import "database/sql"

// Migration now includes a Dependencies method.
type Migration interface {
	Run(db *sql.DB) error
	Dependencies() []int
}

// GoMigrationRegistry now holds the Migration interface, which includes dependencies.
var GoMigrationRegistry = make(map[string]Migration)

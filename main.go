// main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"migrate/core"
	postgres "migrate/drivers/database"
	file "migrate/drivers/source"
	_ "migrate/migrations"
)

func main() {
	action := flag.String("action", "", "one of: up, down, goto, create, status")
	path := flag.String("path", "migrations", "directory for migration files")
	dbURL := flag.String("db", "", "database connection string (required for up/down/goto/status)")
	name := flag.String("name", "", "migration name for create action")
	version := flag.Int("version", 0, "target migration version for goto action")
	flag.Parse()

	switch *action {
	case "create":
		if *name == "" {
			log.Fatal("must provide -name for create action")
		}
		if err := createMigration(*name, *path); err != nil {
			log.Fatalf("create migration failed: %v", err)
		}
		fmt.Println("Migration files created successfully.")

	case "up", "down", "goto", "status":
		if *dbURL == "" {
			log.Fatal("must provide -db for migration actions")
		}
		runMigration(*action, *path, *dbURL, *version)

	default:
		log.Fatalf("unknown action: %s. Use one of: up, down, goto, create, status", *action)
	}
}

func runMigration(action, path, dbURL string, version int) {
	db, err := postgres.New(dbURL)
	if err != nil {
		log.Fatalf("db connection error: %v", err)
	}

	src, err := file.New(path)
	if err != nil {
		log.Fatalf("file source error: %v", err)
	}

	migrator := core.New(src, db)

	switch action {
	case "up":
		logFatal(migrator.Up(), "migrate up failed")
	case "down":
		logFatal(migrator.Down(), "migrate down failed")
	case "goto":
		if version <= 0 {
			log.Fatal("the -version flag must be positive for 'goto'")
		}
		logFatal(migrator.Goto(version), "migrate goto failed")
	case "status":
		logFatal(migrator.Status(), "migrate status failed")
	}
}

func logFatal(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %v", msg, err)
	}
}

func createMigration(name, dir string) error {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	maxVersion := 0
	for _, file := range files {
		var ver int
		if _, err := fmt.Sscanf(file.Name(), "%06d_", &ver); err == nil && ver > maxVersion {
			maxVersion = ver
		}
	}

	newVersion := maxVersion + 1
	for _, ext := range []string{".up.sql", ".down.sql"} {
		path := fmt.Sprintf("%s/%06d_%s%s", dir, newVersion, name, ext)
		content := fmt.Sprintf("-- Write your %s migration SQL here\n", strings.TrimSuffix(ext, ".sql"))
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
		fmt.Println("Created migration file:", path)
	}
	return nil
}

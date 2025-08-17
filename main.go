// main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/PawanMugalihalli/migrate-cli/core"
	postgres "github.com/PawanMugalihalli/migrate-cli/drivers/database"
	file "github.com/PawanMugalihalli/migrate-cli/drivers/source"
	_ "github.com/PawanMugalihalli/migrate-cli/migrations"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

var (
	// This single metric will replace both the counter and the old histogram.
	commandDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "migrate_command_duration_seconds",
		Help: "The duration of each CLI command.",
	},
		// These are the labels we will use to break down the data.
		[]string{"command", "status"},
	)
)

func main() {
	// migrationsDir := "./migrations"
	// totalMigrations := 1000

	// log.Printf("Generating %d migration file pairs in '%s'...", totalMigrations, migrationsDir)

	// if err := os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
	// 	log.Fatalf("Failed to create migrations directory: %v", err)
	// }

	// for i := 9; i <= totalMigrations; i++ {
	// 	// --- Create the 'up' file ---
	// 	upFilename := fmt.Sprintf("%06d_auto_generated_migration_%d.up.sql", i, i)
	// 	upContent := fmt.Sprintf("-- UP migration for version %d\n", i)
	// 	err := os.WriteFile(filepath.Join(migrationsDir, upFilename), []byte(upContent), 0644)
	// 	if err != nil {
	// 		log.Fatalf("Failed to write up file for version %d: %v", i, err)
	// 	}

	// 	// --- Create the 'down' file ---
	// 	downFilename := fmt.Sprintf("%06d_auto_generated_migration_%d.down.sql", i, i)
	// 	downContent := fmt.Sprintf("-- DOWN migration for version %d\n", i)
	// 	err = os.WriteFile(filepath.Join(migrationsDir, downFilename), []byte(downContent), 0644)
	// 	if err != nil {
	// 		log.Fatalf("Failed to write down file for version %d: %v", i, err)
	// 	}
	// }

	// log.Printf("Successfully generated %d migration file pairs.", totalMigrations)

	action := flag.String("action", "", "one of: up, down, goto, create, status")
	path := flag.String("path", "migrations", "directory for migration files")
	dbURL := flag.String("db", "", "database connection string (required for up/down/goto/status)")
	name := flag.String("name", "", "migration name for create action")
	version := flag.Int("version", 0, "target migration version for goto action")
	flag.Parse()

	status := "success"
	startTime := time.Now()

	defer func() {
		// This defer runs at the very end.
		// It records the duration with the correct command and status labels.
		duration := time.Since(startTime).Seconds()
		commandDuration.WithLabelValues(*action, status).Observe(duration)

		pushMetrics()
	}()

	log.Printf("Executing action: %s", *action)

	switch *action {
	case "create":
		if *name == "" {
			status = "failure"
			log.Fatal("must provide -name for create action")
		}
		if err := createMigration(*name, *path); err != nil {
			status = "failure"
			log.Fatalf("create migration failed: %v", err)
		}
		fmt.Println("Migration files created successfully.")

	case "up", "down", "goto", "status":
		if *dbURL == "" {
			status = "failure"
			log.Fatal("must provide -db for migration actions")
		}
		runMigration(*action, *path, *dbURL, *version)

	default:
		log.Fatalf("unknown action: %s. Use one of: up, down, goto, create, status", *action)
	}
	// identifier := "auto_generated_migration" // The unique part of the filename to look for
	// deleteCount := 0

	// log.Printf("Scanning '%s' for files to delete...", migrationsDir)

	// // Read all files in the directory
	// files, err := os.ReadDir(migrationsDir)
	// if err != nil {
	// 	log.Fatalf("Failed to read migrations directory: %v", err)
	// }

	// for _, file := range files {
	// 	// Check if the filename contains our specific identifier
	// 	if strings.Contains(file.Name(), identifier) {
	// 		filePath := filepath.Join(migrationsDir, file.Name())
	// 		if err := os.Remove(filePath); err != nil {
	// 			log.Printf("Failed to delete file %s: %v", filePath, err)
	// 			continue
	// 		}
	// 		// log.Printf("Deleted: %s", file.Name())
	// 		deleteCount++
	// 	}
	// }

	// log.Printf("Cleanup complete. Deleted %d files.", deleteCount)
}

// 2. Create the function to push metrics
func pushMetrics() {
	pusher := push.New("http://pushgateway:9091", "migrate_cli")

	// We only need to register the one HistogramVec.
	pusher.Collector(commandDuration)

	log.Println("Pushing metrics to Pushgateway...")
	if err := pusher.Push(); err != nil {
		log.Printf("Could not push metrics to Pushgateway: %v", err)
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

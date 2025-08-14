package file

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"migrate/core"
)

// FileSource reads migration files from a directory.
type FileSource struct {
	dir string
}

func New(dir string) (*FileSource, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("migration directory not found: %s", dir)
	}
	return &FileSource{dir: dir}, nil
}

// Regex to match migration files like "000001_create_users.up.sql".
var migrationFileRegex = regexp.MustCompile(`^(\d{6})_(.+)\.(up|down)\.(sql|go)$`)

// file/source.go

// ListMigrations scans the directory for migration files and returns their metadata.
func (f *FileSource) ListMigrations() ([]core.MigrationMetadata, error) {
	files, err := os.ReadDir(f.dir)
	if err != nil {
		return nil, fmt.Errorf("could not read migration directory: %w", err)
	}

	// Use a map to collect and combine data for each version number.
	migrationsMap := make(map[int]core.MigrationMetadata)
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		matches := migrationFileRegex.FindStringSubmatch(name)
		if matches == nil {
			continue
		}

		version, _ := strconv.Atoi(matches[1])
		migName := matches[2]
		direction := matches[3]

		// Find or create the metadata entry for this version
		metadata := migrationsMap[version]
		metadata.Version = version
		metadata.Name = migName

		// Set the correct file field based on the direction
		if direction == "up" {
			metadata.UpFile = name
			// Only parse dependencies from 'up' files to avoid redundant parsing.
			deps, err := parseDependencies(filepath.Join(f.dir, name))
			if err != nil {
				return nil, fmt.Errorf("failed to parse dependencies for %s: %w", name, err)
			}
			metadata.Dependencies = deps
		} else if direction == "down" {
			metadata.DownFile = name
		}

		// Put the updated struct back in the map.
		migrationsMap[version] = metadata
	}

	// Convert map to slice for sorting.
	migrations := make([]core.MigrationMetadata, 0, len(migrationsMap))
	for _, m := range migrationsMap {
		migrations = append(migrations, m)
	}

	// Sort by version for predictable order.
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// ReadUp finds and reads the content of an 'up' migration file.
func (f *FileSource) ReadUp(version int) (string, error) {
	pattern := fmt.Sprintf("%s/%06d_*.up.sql", f.dir, version)
	return readFirstMatch(pattern)
}

// ReadDown finds and reads the content of a 'down' migration file.
func (f *FileSource) ReadDown(version int) (string, error) {
	pattern := fmt.Sprintf("%s/%06d_*.down.sql", f.dir, version)
	return readFirstMatch(pattern)
}

// Helper to read the first file matching a glob pattern.
func readFirstMatch(pattern string) (string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no migration file found for pattern: %s", pattern)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", matches[0], err)
	}
	return string(data), nil
}

// parseDependencies scans the top of a file for dependency comments.
// Example: -- depends_on: 000001_create_users, 000002
func parseDependencies(path string) ([]int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var deps []int
	scanner := bufio.NewScanner(file)
	depRegex := regexp.MustCompile(`(?i)^--\s*depends_on:\s*(.+)`)
	numRegex := regexp.MustCompile(`^\d{6}`)

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		// Stop scanning if we hit a non-comment or an empty line.
		if !strings.HasPrefix(trimmedLine, "--") || trimmedLine == "--" {
			break
		}

		matches := depRegex.FindStringSubmatch(trimmedLine)
		if len(matches) < 2 {
			continue
		}

		depTokens := strings.Split(matches[1], ",")
		for _, token := range depTokens {
			cleanToken := strings.TrimSpace(token)
			if numStr := numRegex.FindString(cleanToken); numStr != "" {
				v, _ := strconv.Atoi(numStr)
				deps = append(deps, v)
			}
		}
		// Once we've found and parsed a depends_on line, we can stop.
		break
	}

	return deps, scanner.Err()
}

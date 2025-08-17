package file

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"migrate/core"
	"migrate/gomigration"
)

// The struct and New function remain the same.
type FileSource struct {
	dir string
}

func New(dir string) (*FileSource, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("migration directory not found: %s", dir)
	}
	return &FileSource{dir: dir}, nil
}

var migrationFileRegex = regexp.MustCompile(`^(\d{6})_(.+)\.(up|down)\.(sql|go)$`)

// parseResult remains the same.
type parseResult struct {
	metadata core.MigrationMetadata
	err      error
}

// ListMigrations now only handles explicit dependencies for both SQL and Go.
func (f *FileSource) ListMigrations() ([]core.MigrationMetadata, error) {
	files, err := os.ReadDir(f.dir)
	if err != nil {
		return nil, fmt.Errorf("could not read migration directory: %w", err)
	}

	// Concurrent parsing logic remains the same.
	numWorkers := runtime.NumCPU()
	jobs := make(chan os.DirEntry, len(files))
	results := make(chan parseResult, len(files))
	var wg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go f.parserWorker(&wg, jobs, results)
	}

	for _, file := range files {
		if !file.IsDir() {
			jobs <- file
		}
	}
	close(jobs)

	wg.Wait()
	close(results)

	migrationsMap := make(map[int]core.MigrationMetadata)
	for res := range results {
		if res.err != nil {
			return nil, res.err
		}
		m := res.metadata
		metadata := migrationsMap[m.Version]
		metadata.Version = m.Version
		metadata.Name = m.Name

		if m.UpFile != "" {
			metadata.UpFile = m.UpFile
			metadata.Dependencies = m.Dependencies
		}
		if m.DownFile != "" {
			metadata.DownFile = m.DownFile
		}
		migrationsMap[m.Version] = metadata
	}

	// Convert map to slice for sorting.
	migrations := make([]core.MigrationMetadata, 0, len(migrationsMap))
	for _, m := range migrationsMap {
		migrations = append(migrations, m)
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// parserWorker is updated to fetch dependencies for Go migrations.
func (f *FileSource) parserWorker(wg *sync.WaitGroup, jobs <-chan os.DirEntry, results chan<- parseResult) {
	defer wg.Done()
	for file := range jobs {
		name := file.Name()
		matches := migrationFileRegex.FindStringSubmatch(name)
		if matches == nil {
			continue
		}

		version, _ := strconv.Atoi(matches[1])
		migName := matches[2]
		direction := matches[3]
		fileType := matches[4]

		metadata := core.MigrationMetadata{
			Version: version,
			Name:    migName,
		}

		var deps []int
		var err error

		if direction == "up" {
			metadata.UpFile = name
			if fileType == "sql" {
				// SQL files: parse dependencies from comments.
				deps, err = parseDependencies(filepath.Join(f.dir, name))
				if err != nil {
					results <- parseResult{err: fmt.Errorf("failed to parse dependencies for %s: %w", name, err)}
					return
				}
			} else if fileType == "go" {
				// Go files: get dependencies from the registered migration.
				goMig, exists := gomigration.GoMigrationRegistry[name]
				if !exists {
					// NOTE: This assumes Go migrations are registered before this function runs.
					// An empty slice will be used if not found, which is safe.
					deps = []int{}
				} else {
					deps = goMig.Dependencies()
				}
			}
			metadata.Dependencies = deps
		} else if direction == "down" {
			metadata.DownFile = name
		}

		results <- parseResult{metadata: metadata}
	}
}

// ReadUp, ReadDown, and readFirstMatch remain the same.
func (f *FileSource) ReadUp(version int) (string, error) {
	// ... (no changes)
	pattern := fmt.Sprintf("%s/%06d_*.up.sql", f.dir, version)
	return readFirstMatch(pattern)
}

func (f *FileSource) ReadDown(version int) (string, error) {
	// ... (no changes)
	pattern := fmt.Sprintf("%s/%06d_*.down.sql", f.dir, version)
	return readFirstMatch(pattern)
}

func readFirstMatch(pattern string) (string, error) {
	// ... (no changes)
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

// parseDependencies now returns an empty slice if no explicit dependency line is found.
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
		// Once we find the line, we can stop scanning.
		return deps, scanner.Err()
	}

	// If the loop finishes without finding a 'depends_on' line, return an empty slice.
	return []int{}, scanner.Err()
}

package core

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

//-------------------------------------------------
// 1. MOCK IMPLEMENTATIONS
// These are "fake" versions of interfaces for testing.
//-------------------------------------------------

// MockSource is a mock implementation of the Source interface.
type MockSource struct {
	MigrationsToReturn []MigrationMetadata
	SQLToReturn        string
	ErrorToReturn      error
}

func (ms *MockSource) ListMigrations() ([]MigrationMetadata, error) {
	return ms.MigrationsToReturn, ms.ErrorToReturn
}
func (ms *MockSource) ReadUp(version int) (string, error)   { return ms.SQLToReturn, nil }
func (ms *MockSource) ReadDown(version int) (string, error) { return "", nil }

// MockDatabase is a mock implementation of the Database interface.
type MockDatabase struct {
	AppliedMigrations map[string]bool
	StatesSet         map[string]bool
	SQLRan            []string
}

func (md *MockDatabase) Run(sql string) error {
	md.SQLRan = append(md.SQLRan, sql)
	return nil
}
func (md *MockDatabase) GetApplied(direction string) (map[string]bool, error) {
	if md.AppliedMigrations == nil {
		return make(map[string]bool), nil
	}
	return md.AppliedMigrations, nil
}
func (md *MockDatabase) SetMigrationState(versionName string, dirty bool, direction string) error {
	if md.StatesSet == nil {
		md.StatesSet = make(map[string]bool)
	}
	md.StatesSet[versionName] = dirty
	return nil
}
func (md *MockDatabase) Version() (int, bool, error) { return 0, false, nil }
func (md *MockDatabase) DB() *sql.DB                 { return nil }

//-------------------------------------------------
// 2. TEST CASE
// This is the test function you provided.
//-------------------------------------------------

func TestMigrator_Up_WithManyMigrations(t *testing.T) {
	// --- Arrange ---
	// t.Log("Arranging test with 10 migrations...")
	totalMigrations := 1000
	mockMigrations := make([]MigrationMetadata, totalMigrations)
	for i := 0; i < totalMigrations; i++ {
		version := i + 1
		mockMigrations[i] = MigrationMetadata{
			Version: version,
			Name:    fmt.Sprintf("migration_%d", version),
			UpFile:  fmt.Sprintf("%06d_migration_%d.up.sql", version, version),
		}
	}

	alreadyAppliedCount := 50
	alreadyApplied := make(map[string]bool)
	for i := 0; i < alreadyAppliedCount; i++ {
		alreadyApplied[mockMigrations[i].UpFile] = true
	}

	mockSource := &MockSource{MigrationsToReturn: mockMigrations}
	mockDB := &MockDatabase{
		AppliedMigrations: alreadyApplied,
		StatesSet:         make(map[string]bool),
	}
	migrator := New(mockSource, mockDB)

	// --- Act ---
	// t.Log("Act: Running the Up() command...")
	err := migrator.Up()

	// --- Assert ---
	// t.Log("Asserting results...")

	if err != nil {
		t.Fatalf("Up() returned an unexpected error: %v", err)
	}

	expectedRuns := totalMigrations - alreadyAppliedCount // 10 - 3 = 7
	if len(mockDB.SQLRan) != expectedRuns {
		t.Errorf("expected %d SQL commands to be run, but got %d", expectedRuns, len(mockDB.SQLRan))
	}

	// --- THIS SECTION IS CORRECTED ---
	// Spot-check the first pending migration (version 4 at index 3) and the last one (version 10 at index 9).
	firstPendingFile := mockMigrations[50].UpFile // Corrected: Check version 4 (index 3)
	lastPendingFile := mockMigrations[999].UpFile // Corrected: Comment for clarity

	if _, ok := mockDB.StatesSet[firstPendingFile]; !ok {
		t.Errorf("expected migration '%s' to have its state set, but it wasn't", firstPendingFile)
	}

	if _, ok := mockDB.StatesSet[lastPendingFile]; !ok {
		t.Errorf("expected migration '%s' to have its state set, but it wasn't", lastPendingFile)
	}

	// Verify that an already-applied migration (e.g., version 1 at index 0) was not touched.
	firstAppliedFile := mockMigrations[0].UpFile
	if _, ok := mockDB.StatesSet[firstAppliedFile]; ok {
		t.Errorf("already applied migration '%s' was run unexpectedly", firstAppliedFile)
	}
}

// In core/migrator_test.go

func TestTopoSort(t *testing.T) {
	// --- Arrange ---
	// Create a set of migrations where version 3 depends on 1 and 2.
	// The input is deliberately unsorted to test the sorting logic.
	migrations := []MigrationMetadata{
		{Version: 3, Name: "C", Dependencies: []int{1, 2}},
		{Version: 1, Name: "A"},
		{Version: 2, Name: "B"},
	}

	// --- Act ---
	// Run the function we want to test.
	sorted, err := TopoSort(migrations)

	// --- Assert ---
	// Verify that the output is correct.
	if err != nil {
		t.Fatalf("TopoSort() returned an unexpected error: %v", err)
	}

	if len(sorted) != 3 {
		t.Fatalf("expected 3 sorted migrations, but got %d", len(sorted))
	}

	// The most important check: version 3 must come after its dependencies.
	// In this sorted list, it must be the last element.
	if sorted[2].Version != 3 {
		t.Errorf("expected migration 3 to be last in the sorted list, but its version was %d", sorted[2].Version)
	}

	// We can also verify that the first two elements are not version 3.
	if sorted[0].Version == 3 || sorted[1].Version == 3 {
		t.Error("migration 3 appeared before one of its dependencies")
	}
}
func TestTopoSort_DirectCycle(t *testing.T) {
	// Arrange: A direct cycle between 1 and 2.
	migrations := []MigrationMetadata{
		{Version: 1, Name: "A", Dependencies: []int{2}},
		{Version: 2, Name: "B", Dependencies: []int{1}},
	}

	// Act
	_, err := TopoSort(migrations)

	// Assert
	if err == nil {
		t.Fatal("TopoSort() did not return an error for a direct cycle")
	}

	expectedError := "cycle detected"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("expected error message to contain '%s', but got '%v'", expectedError, err)
	}
}
func TestTopoSort_IndirectCycle(t *testing.T) {
	// Arrange: An indirect cycle.
	migrations := []MigrationMetadata{
		{Version: 1, Name: "A", Dependencies: []int{3}},
		{Version: 2, Name: "B", Dependencies: []int{1}},
		{Version: 3, Name: "C", Dependencies: []int{2}},
	}

	// Act
	_, err := TopoSort(migrations)

	// Assert
	if err == nil {
		t.Fatal("TopoSort() did not return an error for an indirect cycle")
	}

	expectedError := "cycle detected"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("expected error message to contain '%s', but got '%v'", expectedError, err)
	}
}

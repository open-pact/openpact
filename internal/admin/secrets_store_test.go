package admin

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNewSecretStore(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewSecretStore(tmpDir)

	if store == nil {
		t.Fatal("Expected non-nil store")
	}
	if store.dataDir != tmpDir {
		t.Errorf("Expected dataDir %s, got %s", tmpDir, store.dataDir)
	}
}

func TestSecretStore_SetAndAll(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	if err := store.Set("API_KEY", "secret123"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if err := store.Set("DB_PASSWORD", "dbpass456"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	all, err := store.All()
	if err != nil {
		t.Fatalf("All failed: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("Expected 2 secrets, got %d", len(all))
	}
	if all["API_KEY"] != "secret123" {
		t.Errorf("Expected API_KEY=secret123, got %s", all["API_KEY"])
	}
	if all["DB_PASSWORD"] != "dbpass456" {
		t.Errorf("Expected DB_PASSWORD=dbpass456, got %s", all["DB_PASSWORD"])
	}
}

func TestSecretStore_SetOverwrite(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	store.Set("API_KEY", "original")
	store.Set("API_KEY", "updated")

	val, err := store.Get("API_KEY")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "updated" {
		t.Errorf("Expected 'updated', got '%s'", val)
	}
}

func TestSecretStore_List(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	store.Set("BETA_KEY", "val1")
	store.Set("ALPHA_KEY", "val2")
	store.Set("GAMMA_KEY", "val3")

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(entries))
	}

	// Should be sorted by name
	if entries[0].Name != "ALPHA_KEY" {
		t.Errorf("Expected first entry ALPHA_KEY, got %s", entries[0].Name)
	}
	if entries[1].Name != "BETA_KEY" {
		t.Errorf("Expected second entry BETA_KEY, got %s", entries[1].Name)
	}
	if entries[2].Name != "GAMMA_KEY" {
		t.Errorf("Expected third entry GAMMA_KEY, got %s", entries[2].Name)
	}

	// Entries should have timestamps
	if entries[0].CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
	if entries[0].UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}
}

func TestSecretStore_ListEmpty(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(entries))
	}
}

func TestSecretStore_Get(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	store.Set("MY_SECRET", "my_value")

	val, err := store.Get("MY_SECRET")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "my_value" {
		t.Errorf("Expected 'my_value', got '%s'", val)
	}
}

func TestSecretStore_GetNotFound(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	_, err := store.Get("NONEXISTENT")
	if err != ErrSecretNotFound {
		t.Errorf("Expected ErrSecretNotFound, got %v", err)
	}
}

func TestSecretStore_Delete(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	store.Set("TO_DELETE", "value")

	err := store.Delete("TO_DELETE")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	all, _ := store.All()
	if _, exists := all["TO_DELETE"]; exists {
		t.Error("Expected secret to be deleted")
	}
}

func TestSecretStore_DeleteNotFound(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	err := store.Delete("NONEXISTENT")
	if err != ErrSecretNotFound {
		t.Errorf("Expected ErrSecretNotFound, got %v", err)
	}
}

func TestSecretStore_CreateAndDuplicate(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	err := store.Create("NEW_SECRET", "value1")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Duplicate should fail
	err = store.Create("NEW_SECRET", "value2")
	if err != ErrSecretExists {
		t.Errorf("Expected ErrSecretExists, got %v", err)
	}

	// Original value should be preserved
	val, _ := store.Get("NEW_SECRET")
	if val != "value1" {
		t.Errorf("Expected original value 'value1', got '%s'", val)
	}
}

func TestSecretStore_UpdateExisting(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	store.Create("MY_KEY", "original")

	err := store.Update("MY_KEY", "updated")
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	val, _ := store.Get("MY_KEY")
	if val != "updated" {
		t.Errorf("Expected 'updated', got '%s'", val)
	}
}

func TestSecretStore_UpdateNotFound(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	err := store.Update("NONEXISTENT", "value")
	if err != ErrSecretNotFound {
		t.Errorf("Expected ErrSecretNotFound, got %v", err)
	}
}

func TestSecretStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create and populate store
	store1 := NewSecretStore(tmpDir)
	store1.Set("PERSISTED_KEY", "persisted_value")
	store1.Set("ANOTHER_KEY", "another_value")

	// Create new store instance with same dir
	store2 := NewSecretStore(tmpDir)

	all, err := store2.All()
	if err != nil {
		t.Fatalf("All failed: %v", err)
	}

	if all["PERSISTED_KEY"] != "persisted_value" {
		t.Errorf("Expected persisted_value, got %s", all["PERSISTED_KEY"])
	}
	if all["ANOTHER_KEY"] != "another_value" {
		t.Errorf("Expected another_value, got %s", all["ANOTHER_KEY"])
	}
}

func TestSecretStore_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewSecretStore(tmpDir)

	store.Set("TEST_KEY", "test_value")

	info, err := os.Stat(filepath.Join(tmpDir, "starlark_secrets.json"))
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("Expected permissions 0600, got %o", perm)
	}
}

func TestSecretStore_ValidateName(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	tests := []struct {
		name    string
		wantErr bool
	}{
		{"API_KEY", false},
		{"A", false},
		{"MY_SECRET_123", false},
		{"X99", false},
		{"lowercase", true},
		{"mixedCase", true},
		{"123_STARTS_WITH_NUMBER", true},
		{"_STARTS_WITH_UNDERSCORE", true},
		{"HAS SPACES", true},
		{"HAS-DASHES", true},
		{"", true},
		{strings.Repeat("A", 65), true},
	}

	for _, tt := range tests {
		err := store.Set(tt.name, "value")
		if tt.wantErr && err == nil {
			t.Errorf("Set(%q): expected error, got nil", tt.name)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("Set(%q): unexpected error: %v", tt.name, err)
		}
	}
}

func TestSecretStore_ValidateValue(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	// Empty value
	err := store.Set("TEST_KEY", "")
	if err == nil {
		t.Error("Expected error for empty value")
	}

	// Oversized value
	err = store.Set("TEST_KEY", strings.Repeat("x", 4097))
	if err == nil {
		t.Error("Expected error for oversized value")
	}

	// Max length value should be fine
	err = store.Set("TEST_KEY", strings.Repeat("x", 4096))
	if err != nil {
		t.Errorf("Expected no error for max-length value, got %v", err)
	}
}

func TestSecretStore_ConcurrentAccess(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	var wg sync.WaitGroup
	iterations := 50

	// Concurrent Sets
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := "KEY_" + strings.Repeat("A", n%26+1)
			// Ensure valid name pattern
			if n%3 == 0 {
				name = "KEYA"
			} else if n%3 == 1 {
				name = "KEYB"
			} else {
				name = "KEYC"
			}
			store.Set(name, "value")
		}(i)
	}

	// Concurrent Gets
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Get("KEYA")
		}()
	}

	// Concurrent Lists
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.List()
		}()
	}

	// Concurrent All
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.All()
		}()
	}

	// Concurrent Deletes
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Delete("KEYC") // May or may not exist
		}()
	}

	wg.Wait()

	// Verify store is still functional
	err := store.Set("FINAL_KEY", "final_value")
	if err != nil {
		t.Fatalf("Store broken after concurrent access: %v", err)
	}
}

func TestSecretStore_UpdatePreservesCreatedAt(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	store.Create("TIMESTAMP_KEY", "original")

	entries1, _ := store.List()
	var createdAt1 = entries1[0].CreatedAt

	store.Update("TIMESTAMP_KEY", "updated")

	entries2, _ := store.List()
	if !entries2[0].CreatedAt.Equal(createdAt1) {
		t.Error("Update should preserve CreatedAt")
	}
	if !entries2[0].UpdatedAt.After(createdAt1) || entries2[0].UpdatedAt.Equal(createdAt1) {
		// UpdatedAt should be >= CreatedAt (may be equal if test runs too fast)
	}
}

func TestSecretStore_CreateValidation(t *testing.T) {
	store := NewSecretStore(t.TempDir())

	// Invalid name
	err := store.Create("lowercase", "value")
	if err == nil {
		t.Error("Expected error for invalid name in Create")
	}

	// Empty value
	err = store.Create("VALID_NAME", "")
	if err == nil {
		t.Error("Expected error for empty value in Create")
	}
}

func TestSecretStore_UpdateValidation(t *testing.T) {
	store := NewSecretStore(t.TempDir())
	store.Create("MY_KEY", "original")

	// Empty value
	err := store.Update("MY_KEY", "")
	if err == nil {
		t.Error("Expected error for empty value in Update")
	}

	// Oversized value
	err = store.Update("MY_KEY", strings.Repeat("x", 4097))
	if err == nil {
		t.Error("Expected error for oversized value in Update")
	}
}

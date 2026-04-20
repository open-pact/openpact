package admin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-pact/openpact/internal/storage"
)

// newScriptStore builds a ScriptStore backed by an in-memory SQLite DB
// and a per-test scripts directory. Replaces the old (scriptsDir,
// dataDir) constructor now that approval metadata lives in op_approvals.
func newScriptStore(t *testing.T, allowlist []string) (*ScriptStore, string) {
	t.Helper()
	scriptsDir := filepath.Join(t.TempDir(), "scripts")
	store, err := NewScriptStore(storage.NewTestDB(t), scriptsDir, allowlist)
	if err != nil {
		t.Fatalf("NewScriptStore failed: %v", err)
	}
	return store, scriptsDir
}

func TestScriptStore_CreateAndGet(t *testing.T) {
	store, _ := newScriptStore(t, nil)

	source := `# @description: Test script
# @secrets: API_KEY

def main():
    return "hello"
`

	script, err := store.Create("test.star", source, "admin")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if script.Name != "test.star" {
		t.Errorf("Expected name 'test.star', got '%s'", script.Name)
	}

	if script.Status != StatusPending {
		t.Errorf("Expected status 'pending', got '%s'", script.Status)
	}

	if script.Description != "Test script" {
		t.Errorf("Expected description 'Test script', got '%s'", script.Description)
	}

	if len(script.RequiredSecrets) != 1 || script.RequiredSecrets[0] != "API_KEY" {
		t.Errorf("Expected secrets [API_KEY], got %v", script.RequiredSecrets)
	}

	// Get with source
	script, err = store.Get("test.star", true)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if script.Source != source {
		t.Error("Expected source to be included")
	}

	// Get without source
	script, err = store.Get("test.star", false)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if script.Source != "" {
		t.Error("Expected source to be empty")
	}
}

func TestScriptStore_List(t *testing.T) {
	store, _ := newScriptStore(t, nil)

	store.Create("script1.star", "def main(): pass", "admin")
	store.Create("script2.star", "def main(): pass", "admin")
	store.Create("script3.star", "def main(): pass", "admin")

	scripts, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(scripts) != 3 {
		t.Errorf("Expected 3 scripts, got %d", len(scripts))
	}
}

func TestScriptStore_Update(t *testing.T) {
	store, _ := newScriptStore(t, nil)

	store.Create("test.star", "def main(): return 1", "admin")

	// Approve it first
	store.Approve("test.star", "admin")

	// Get the approved script
	script, _ := store.Get("test.star", false)
	if script.Status != StatusApproved {
		t.Error("Expected script to be approved")
	}

	// Update the script
	script, err := store.Update("test.star", "def main(): return 2", "admin")
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Should be back to pending
	if script.Status != StatusPending {
		t.Errorf("Expected status 'pending' after update, got '%s'", script.Status)
	}
}

func TestScriptStore_Delete(t *testing.T) {
	store, _ := newScriptStore(t, nil)

	store.Create("test.star", "def main(): pass", "admin")

	err := store.Delete("test.star")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get("test.star", false)
	if err != ErrScriptNotFound {
		t.Errorf("Expected ErrScriptNotFound, got %v", err)
	}

	// Delete non-existent
	err = store.Delete("nonexistent.star")
	if err != ErrScriptNotFound {
		t.Errorf("Expected ErrScriptNotFound, got %v", err)
	}
}

func TestScriptStore_ApproveAndReject(t *testing.T) {
	store, _ := newScriptStore(t, nil)

	store.Create("test.star", "def main(): pass", "admin")

	// Approve
	script, err := store.Approve("test.star", "admin")
	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}

	if script.Status != StatusApproved {
		t.Errorf("Expected status 'approved', got '%s'", script.Status)
	}

	if script.ApprovedBy != "admin" {
		t.Errorf("Expected approved_by 'admin', got '%s'", script.ApprovedBy)
	}

	if script.ApprovedAt == nil {
		t.Error("Expected approved_at to be set")
	}

	// Reject
	script, err = store.Reject("test.star", "admin", "Security concern")
	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	if script.Status != StatusRejected {
		t.Errorf("Expected status 'rejected', got '%s'", script.Status)
	}

	if script.RejectReason != "Security concern" {
		t.Errorf("Expected reject_reason 'Security concern', got '%s'", script.RejectReason)
	}
}

func TestScriptStore_CanExecute(t *testing.T) {
	store, _ := newScriptStore(t, nil)

	store.Create("test.star", "def main(): pass", "admin")

	// Pending script cannot execute
	err := store.CanExecute("test.star")
	if err != ErrScriptNotApproved {
		t.Errorf("Expected ErrScriptNotApproved, got %v", err)
	}

	// Approve it
	store.Approve("test.star", "admin")

	// Approved script can execute
	err = store.CanExecute("test.star")
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	// Modify the script (changes hash)
	store.Update("test.star", "def main(): return 'modified'", "admin")

	// Modified script cannot execute
	err = store.CanExecute("test.star")
	if err != ErrScriptNotApproved {
		t.Errorf("Expected ErrScriptNotApproved (script modified), got %v", err)
	}
}

func TestScriptStore_Allowlist(t *testing.T) {
	store, scriptsDir := newScriptStore(t, []string{"trusted.star"})

	// Create a script directly on disk (simulating pre-existing trusted script)
	os.WriteFile(filepath.Join(scriptsDir, "trusted.star"), []byte("def main(): pass"), 0644)

	// Allowlisted script should be approved automatically
	script, _ := store.Get("trusted.star", false)
	if script.Status != StatusApproved {
		t.Errorf("Expected allowlisted script to be approved, got '%s'", script.Status)
	}

	// Allowlisted script can execute without explicit approval
	err := store.CanExecute("trusted.star")
	if err != nil {
		t.Errorf("Expected allowlisted script to be executable, got %v", err)
	}

	if !store.IsAllowlisted("trusted.star") {
		t.Error("Expected IsAllowlisted to return true")
	}

	if store.IsAllowlisted("other.star") {
		t.Error("Expected IsAllowlisted to return false for non-allowlisted script")
	}
}

func TestScriptStore_Persistence(t *testing.T) {
	// Both stores share one in-memory DB and one scripts dir so the
	// second "instance" sees approvals the first instance persisted.
	db := storage.NewTestDB(t)
	scriptsDir := filepath.Join(t.TempDir(), "scripts")

	store1, err := NewScriptStore(db, scriptsDir, nil)
	if err != nil {
		t.Fatalf("NewScriptStore: %v", err)
	}
	store1.Create("test.star", "def main(): pass", "admin")
	store1.Approve("test.star", "admin")

	store2, err := NewScriptStore(db, scriptsDir, nil)
	if err != nil {
		t.Fatalf("NewScriptStore: %v", err)
	}

	script, _ := store2.Get("test.star", false)
	if script.Status != StatusApproved {
		t.Errorf("Expected approval to persist, got status '%s'", script.Status)
	}

	if script.ApprovedBy != "admin" {
		t.Errorf("Expected approved_by to persist, got '%s'", script.ApprovedBy)
	}
}

func TestComputeHash(t *testing.T) {
	hash1 := computeHash("hello world")
	hash2 := computeHash("hello world")
	hash3 := computeHash("different content")

	if hash1 != hash2 {
		t.Error("Same content should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("Different content should produce different hash")
	}

	if len(hash1) != 71 { // "sha256:" + 64 hex chars
		t.Errorf("Expected hash length 71, got %d", len(hash1))
	}
}

func TestParseScriptMetadata(t *testing.T) {
	source := `# @description: Weather fetcher
# @secrets: WEATHER_API_KEY, BACKUP_KEY

def get_weather():
    pass
`

	desc, secrets := parseScriptMetadata(source)

	if desc != "Weather fetcher" {
		t.Errorf("Expected description 'Weather fetcher', got '%s'", desc)
	}

	if len(secrets) != 2 {
		t.Fatalf("Expected 2 secrets, got %d", len(secrets))
	}

	if secrets[0] != "WEATHER_API_KEY" {
		t.Errorf("Expected first secret 'WEATHER_API_KEY', got '%s'", secrets[0])
	}

	if secrets[1] != "BACKUP_KEY" {
		t.Errorf("Expected second secret 'BACKUP_KEY', got '%s'", secrets[1])
	}
}

func TestScriptStore_AutoAddExtension(t *testing.T) {
	store, _ := newScriptStore(t, nil)

	// Create without .star extension
	script, err := store.Create("test", "def main(): pass", "admin")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if script.Name != "test.star" {
		t.Errorf("Expected name 'test.star', got '%s'", script.Name)
	}
}

func TestScriptStore_DuplicateCreate(t *testing.T) {
	store, _ := newScriptStore(t, nil)

	store.Create("test.star", "def main(): pass", "admin")

	_, err := store.Create("test.star", "def main(): pass", "admin")
	if err != ErrScriptExists {
		t.Errorf("Expected ErrScriptExists, got %v", err)
	}
}

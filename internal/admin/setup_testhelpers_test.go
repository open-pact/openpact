package admin

import (
	"database/sql"
	"testing"
	"time"

	"github.com/open-pact/openpact/internal/storage"
	"github.com/open-pact/openpact/internal/storage/users"
)

// testFixture bundles the handler with the backing DB + ai-data dir
// so tests that need to seed setup state, inspect SOUL/USER files, or
// mutate kv rows can do so without reaching into private fields.
type testFixture struct {
	handler   *SetupHandler
	users     *users.Store
	db        *sql.DB
	aiDataDir string
}

// newTestSetupFixture constructs a SetupHandler with a real JWT
// manager backed by a per-test secret (so tests that round-trip the
// refresh cookie can validate the resulting token) and a
// defaultModelSet function that always returns true. Both user
// accounts and setup state live in the shared in-memory DB — no
// filesystem state to clean up.
func newTestSetupFixture(t *testing.T) *testFixture {
	t.Helper()
	aiDataDir := t.TempDir()
	db := storage.NewTestDB(t)
	userStore := users.NewStore(db)
	jwt := NewJWTManager(JWTConfig{
		Secret:        []byte("test-secret-at-least-32-bytes-long-xxxxxxxxxxxxxxxx"),
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 72 * time.Hour,
		Issuer:        "openpact-test",
	})
	return &testFixture{
		handler:   NewSetupHandler(userStore, db, aiDataDir, jwt, false, func() bool { return true }),
		users:     userStore,
		db:        db,
		aiDataDir: aiDataDir,
	}
}

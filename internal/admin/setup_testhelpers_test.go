package admin

import (
	"testing"
	"time"
)

// newTestSetupHandler constructs a SetupHandler with a real JWT
// manager backed by a per-test secret (so tests that round-trip the
// refresh cookie can validate the resulting token) and a
// defaultModelSet function that always returns true. Tests that want
// to exercise the "no default model" branch of Provider can override
// `handler.defaultModelSet` directly.
func newTestSetupHandler(t *testing.T, users *UserStore, dir string) *SetupHandler {
	t.Helper()
	jwt := NewJWTManager(JWTConfig{
		Secret:        []byte("test-secret-at-least-32-bytes-long-xxxxxxxxxxxxxxxx"),
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 72 * time.Hour,
		Issuer:        "openpact-test",
	})
	return NewSetupHandler(users, dir, dir, jwt, false, func() bool { return true })
}

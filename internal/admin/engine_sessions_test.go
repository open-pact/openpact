package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stack-bound/stackllm/conversation"
	"github.com/stack-bound/stackllm/session"
)

// newEngineSessionsFixture wires the handler against an in-memory session
// store with n fake sessions, each spaced a second apart so the ORDER BY
// updated_at DESC is deterministic.
func newEngineSessionsFixture(t *testing.T, n int) (*EngineSessionsHandler, *session.InMemoryStore) {
	t.Helper()
	store := session.NewInMemoryStore()
	base := time.Now().Add(-time.Duration(n) * time.Second)
	for i := 0; i < n; i++ {
		s := session.New()
		s.Name = "s" + itoa(i)
		s.Created = base.Add(time.Duration(i) * time.Second)
		s.Updated = s.Created
		s.Messages = []conversation.Message{
			{Role: conversation.RoleUser, Blocks: []conversation.Block{{Type: conversation.BlockText, Text: "hi"}}},
		}
		if err := store.Save(nil, s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	h := NewEngineSessionsHandler(func() session.SessionStore { return store })
	return h, store
}

func itoa(i int) string {
	// Tiny helper so tests don't pull strconv.
	if i == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

func TestEngineSessions_List_DefaultsAndTotal(t *testing.T) {
	h, _ := newEngineSessionsFixture(t, 3)

	req := httptest.NewRequest(http.MethodGet, "/api/engine/sessions", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var page sessionsPage
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if page.Total != 3 {
		t.Errorf("Total = %d, want 3", page.Total)
	}
	if len(page.Sessions) != 3 {
		t.Errorf("len(Sessions) = %d, want 3", len(page.Sessions))
	}
	// Must NOT include Messages — the list is metadata-only.
	body := rec.Body.String()
	if strings.Contains(body, `"messages"`) {
		t.Errorf("response should not include messages field: %s", body)
	}
	// Must NOT leak the state map.
	if strings.Contains(body, `"state"`) {
		t.Errorf("response should not include state field: %s", body)
	}
	if page.Limit != session.DefaultListLimit {
		t.Errorf("Limit = %d, want DefaultListLimit=%d", page.Limit, session.DefaultListLimit)
	}
	if page.Offset != 0 {
		t.Errorf("Offset = %d, want 0", page.Offset)
	}
}

func TestEngineSessions_List_Pagination(t *testing.T) {
	h, _ := newEngineSessionsFixture(t, 7)

	req := httptest.NewRequest(http.MethodGet, "/api/engine/sessions?limit=3&offset=3", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var page sessionsPage
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if page.Total != 7 {
		t.Errorf("Total = %d, want 7", page.Total)
	}
	if len(page.Sessions) != 3 {
		t.Errorf("len(Sessions) = %d, want 3", len(page.Sessions))
	}
	if page.Limit != 3 || page.Offset != 3 {
		t.Errorf("Limit/Offset = %d/%d, want 3/3", page.Limit, page.Offset)
	}
}

func TestEngineSessions_List_OffsetPastEnd(t *testing.T) {
	h, _ := newEngineSessionsFixture(t, 3)

	req := httptest.NewRequest(http.MethodGet, "/api/engine/sessions?offset=999", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var page sessionsPage
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if page.Total != 3 {
		t.Errorf("Total = %d, want 3", page.Total)
	}
	if len(page.Sessions) != 0 {
		t.Errorf("len(Sessions) = %d, want 0 (offset past end)", len(page.Sessions))
	}
}

func TestEngineSessions_List_CapsHugeLimit(t *testing.T) {
	// An adversarial ?limit=1000000 should be clamped — the handler caps at
	// 200 so a single request can't drag every row out of SQLite.
	h, _ := newEngineSessionsFixture(t, 3)

	req := httptest.NewRequest(http.MethodGet, "/api/engine/sessions?limit=1000000", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var page sessionsPage
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if page.Limit != 200 {
		t.Errorf("Limit = %d, want cap 200", page.Limit)
	}
}

func TestEngineSessions_List_StoreNotReady(t *testing.T) {
	// Accessor returns nil — simulates a call before SetSessionStore has fired.
	h := NewEngineSessionsHandler(func() session.SessionStore { return nil })

	req := httptest.NewRequest(http.MethodGet, "/api/engine/sessions", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestEngineSessions_List_BadMethod(t *testing.T) {
	h, _ := newEngineSessionsFixture(t, 1)
	req := httptest.NewRequest(http.MethodPost, "/api/engine/sessions", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}


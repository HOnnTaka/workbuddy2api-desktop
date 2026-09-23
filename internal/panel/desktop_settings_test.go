package panel

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDesktopSettingsEndpoints(t *testing.T) {
	p := newTestPanel()

	// GET
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panel/api/desktop_settings", nil)
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// POST
	payload := `{"auto_start":false,"start_minimized":true}`
	recPost := httptest.NewRecorder()
	reqPost := httptest.NewRequest("POST", "/panel/api/desktop_settings", bytes.NewBufferString(payload))
	reqPost.Header.Set("Content-Type", "application/json")
	p.ServeHTTP(recPost, reqPost)
	if recPost.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recPost.Code)
	}

	var res map[string]any
	if err := json.Unmarshal(recPost.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res["ok"] != true || res["start_minimized"] != true {
		t.Fatalf("unexpected response: %v", res)
	}
}

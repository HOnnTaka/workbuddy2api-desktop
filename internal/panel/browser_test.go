package panel

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckUpdateEndpoint(t *testing.T) {
	p := newTestPanel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panel/api/check_update", nil)
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res["ok"] != true {
		t.Fatalf("expected ok=true, got %v", res["ok"])
	}
}

func TestOpenBrowserEndpoint(t *testing.T) {
	p := newTestPanel()

	invalidBodies := []string{
		`{"url": "ftp://example.com"}`,
		`{"url": "file:///c:/test"}`,
		`{"url": "javascript:alert(1)"}`,
		`{"url": ""}`,
	}
	for _, b := range invalidBodies {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/panel/api/open_browser", bytes.NewBufferString(b))
		req.Header.Set("Content-Type", "application/json")
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for %s, got %d", b, rec.Code)
		}
	}
}

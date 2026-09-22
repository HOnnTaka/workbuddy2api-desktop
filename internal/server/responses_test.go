package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponsesEndpointNot404(t *testing.T) {
	h := NewHandler(Config{APIKey: "test-secret"})
	// 测试未鉴权路径，如果路由存在且受 withAuth 保护，应该返回 401 Unauthorized（而不是 404 Not Found）
	req := httptest.NewRequest("POST", "/v1/responses", bytes.NewReader([]byte(`{"model":"cn:deepseek-v4.1-flash"}`)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Fatalf("/v1/responses returned 404, expected 401 or other handled status")
	}
	t.Logf("/v1/responses status: %d (not 404)", rec.Code)

	// 测试 /responses (无 v1 前缀)
	req2 := httptest.NewRequest("POST", "/responses", bytes.NewReader([]byte(`{"model":"cn:deepseek-v4.1-flash"}`)))
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec2.Code == http.StatusNotFound {
		t.Fatalf("/responses returned 404, expected 401 or other handled status")
	}
	t.Logf("/responses status: %d (not 404)", rec2.Code)

	// 测试 /v1/messages
	req3 := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte(`{"model":"cn:deepseek-v4.1-flash"}`)))
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req3)

	if rec3.Code == http.StatusNotFound {
		t.Fatalf("/v1/messages returned 404, expected 401 or other handled status")
	}
	t.Logf("/v1/messages status: %d (not 404)", rec3.Code)
}

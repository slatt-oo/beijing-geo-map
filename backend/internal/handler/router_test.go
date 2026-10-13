package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func get(t *testing.T, h http.Handler, path string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for key, values := range header {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("response is not JSON: %v; body=%s", err, body)
	}
	return m
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, status int, code, message string) []byte {
	t.Helper()
	res := rec.Result()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if res.StatusCode != status {
		t.Fatalf("status = %d, want %d; body=%s", res.StatusCode, status, body)
	}
	errObj, ok := decodeJSON(t, body)["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error object; body=%s", body)
	}
	if errObj["code"] != code || errObj["message"] != message {
		t.Fatalf("error = %v, want code=%q message=%q", errObj, code, message)
	}
	return body
}

func TestHealthOK(t *testing.T) {
	rec := get(t, NewRouter(Deps{DB: fakePinger{}}), "/api/v1/health", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	data, ok := decodeJSON(t, rec.Body.Bytes())["data"].(map[string]any)
	if !ok || data["status"] != "ok" || data["db"] != "ok" {
		t.Fatalf("body = %s, want data.status=ok and data.db=ok", rec.Body.String())
	}
}

func TestHealthReportsUnavailableWhenDatabaseIsDown(t *testing.T) {
	r := NewRouter(Deps{DB: fakePinger{err: errors.New("connection refused")}})

	rec := get(t, r, "/api/v1/health", nil)

	assertErrorResponse(t, rec, http.StatusServiceUnavailable, CodeUnavailable, "数据库不可用")
}

func TestUnknownRouteReturnsUnifiedNotFound(t *testing.T) {
	rec := get(t, NewRouter(Deps{DB: fakePinger{}}), "/api/v1/does-not-exist", nil)

	assertErrorResponse(t, rec, http.StatusNotFound, CodeNotFound, "接口不存在")
}

func TestPanicIsRecoveredAsUnifiedInternalError(t *testing.T) {
	r := NewRouter(Deps{DB: fakePinger{}})
	r.GET("/api/v1/test-panic", func(*gin.Context) { panic("secret detail") })

	rec := get(t, r, "/api/v1/test-panic", http.Header{"Accept-Encoding": {"gzip"}})

	body := assertErrorResponse(t, rec, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	if strings.Contains(string(body), "secret detail") {
		t.Fatalf("panic detail leaked to client: %s", body)
	}
}

func TestRequestContextCarriesFiveSecondTimeout(t *testing.T) {
	r := NewRouter(Deps{DB: fakePinger{}})
	var remaining time.Duration
	var hasDeadline bool
	r.GET("/api/v1/test-deadline", func(c *gin.Context) {
		var deadline time.Time
		deadline, hasDeadline = c.Request.Context().Deadline()
		remaining = time.Until(deadline)
		c.Status(http.StatusNoContent)
	})

	get(t, r, "/api/v1/test-deadline", nil)

	if !hasDeadline {
		t.Fatal("request context has no deadline")
	}
	if remaining <= 4*time.Second || remaining > 5*time.Second {
		t.Fatalf("remaining = %v, want within (4s, 5s]", remaining)
	}
}

func TestGinContextExposesRequestTimeout(t *testing.T) {
	r := NewRouter(Deps{DB: fakePinger{}})
	var remaining time.Duration
	var hasDeadline bool
	r.GET("/api/v1/test-gin-deadline", func(c *gin.Context) {
		var deadline time.Time
		deadline, hasDeadline = c.Deadline()
		remaining = time.Until(deadline)
		c.Status(http.StatusNoContent)
	})

	get(t, r, "/api/v1/test-gin-deadline", nil)

	if !hasDeadline {
		t.Fatal("gin.Context has no deadline; handlers passing c to pgx would lose the request timeout")
	}
	if remaining <= 4*time.Second || remaining > 5*time.Second {
		t.Fatalf("remaining = %v, want within (4s, 5s]", remaining)
	}
}

func TestLargeResponsesAreGzipped(t *testing.T) {
	r := NewRouter(Deps{DB: fakePinger{}})
	payload := strings.Repeat("北京", 1000)
	r.GET("/api/v1/test-large", func(c *gin.Context) { writeData(c, payload) })

	res := get(t, r, "/api/v1/test-large", http.Header{"Accept-Encoding": {"gzip"}}).Result()

	if got := res.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	zr, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatalf("body is not gzip: %v", err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if got := decodeJSON(t, body)["data"]; got != payload {
		t.Fatalf("decompressed data mismatch (got %d bytes)", len(body))
	}
}

func TestSmallResponsesAreNotGzipped(t *testing.T) {
	r := NewRouter(Deps{DB: fakePinger{}})

	res := get(t, r, "/api/v1/health", http.Header{"Accept-Encoding": {"gzip"}}).Result()

	if got := res.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want none for responses under 1 KB", got)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", res.StatusCode, body)
	}
	data, ok := decodeJSON(t, body)["data"].(map[string]any)
	if !ok || data["status"] != "ok" || data["db"] != "ok" {
		t.Fatalf("body = %s, want data.status=ok and data.db=ok", body)
	}
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestSuccessfulHealthChecksAreNotLoggedAtInfo(t *testing.T) {
	logs := captureLogs(t)
	r := NewRouter(Deps{DB: fakePinger{}})

	get(t, r, "/api/v1/health", nil)
	get(t, r, "/api/v1/does-not-exist", nil)

	if strings.Contains(logs.String(), `"path":"/api/v1/health"`) {
		t.Fatalf("successful health check was logged at info level:\n%s", logs)
	}
	if !strings.Contains(logs.String(), `"path":"/api/v1/does-not-exist"`) {
		t.Fatalf("other requests must still be logged at info level:\n%s", logs)
	}
}

func TestFailedHealthChecksAreLoggedAtInfo(t *testing.T) {
	logs := captureLogs(t)
	r := NewRouter(Deps{DB: fakePinger{err: errors.New("connection refused")}})

	get(t, r, "/api/v1/health", nil)

	if !strings.Contains(logs.String(), `"path":"/api/v1/health"`) || !strings.Contains(logs.String(), `"status":503`) {
		t.Fatalf("failed health check must be logged at info level with status 503:\n%s", logs)
	}
}

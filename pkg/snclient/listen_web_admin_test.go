package snclient

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
)

func TestAdminLogLevelOverride(t *testing.T) {
	prev, expiresAt, err := SetLogLevelOverride("info", 0)
	require.NoError(t, err)
	_ = prev
	require.True(t, expiresAt.IsZero())

	hdl := &HandlerWebAdmin{Handler: &HandlerAdmin{}}

	body, err := json.Marshal(map[string]any{
		"level":    "debug",
		"duration": 0.2,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/log/level", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	levelNow, expiresNow := GetLogLevelOverride()
	require.Equal(t, "debug", levelNow)
	require.False(t, expiresNow.IsZero())

	deadline := time.Now().Add(2 * time.Second)
	for {
		levelNow, expiresNow = GetLogLevelOverride()
		if levelNow == "info" && expiresNow.IsZero() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("log level did not restore in time: level=%s expires=%s", levelNow, expiresNow.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestAdminLogFileEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "snclient.log")
	logData := []byte("hello log\nline2\n")
	require.NoError(t, os.WriteFile(logPath, logData, 0o600))

	prevPath := LogFilePath
	t.Cleanup(func() { LogFilePath = prevPath })
	LogFilePath = logPath

	h := &HandlerWebAdmin{Handler: &HandlerAdmin{}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/log/file", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, logData, rec.Body.Bytes())
	require.Contains(t, rec.Header().Get("Content-Disposition"), "snclient.log")
}

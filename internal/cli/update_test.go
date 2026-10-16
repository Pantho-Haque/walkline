package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"walkline/internal/constants"
	"walkline/internal/update"
)

func fakeRelease(t *testing.T, status int, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	old := update.APIURL
	update.APIURL = srv.URL
	t.Cleanup(func() { update.APIURL = old; srv.Close() })
}

func withVersion(t *testing.T, v string) {
	old := constants.Version
	constants.Version = v
	t.Cleanup(func() { constants.Version = old })
}

func TestUpdateCmd_Check(t *testing.T) {
	isolate(t)
	tests := []struct {
		name    string
		current string
		status  int
		body    string
		want    string
		wantErr bool
	}{
		{"up to date", "1.2.3", 200, `{"tag_name":"v1.2.3"}`, "Already up to date", false},
		{"current newer than latest", "2.0.0", 200, `{"tag_name":"v1.2.3"}`, "Already up to date", false},
		{"update available", "1.2.3", 200, `{"tag_name":"v1.3.0"}`, "Update available: 1.2.3 -> 1.3.0", false},
		{"dev build is always older", "dev", 200, `{"tag_name":"v0.1.0"}`, "Update available: dev -> 0.1.0", false},
		{"api error", "1.2.3", 500, `oops`, "", true},
		{"unparseable body", "1.2.3", 200, `not json`, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withVersion(t, tt.current)
			fakeRelease(t, tt.status, tt.body)
			out, err := run(t, UpdateCmd(), "--check")
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v out=%s", err, out)
			}
			if !strings.Contains(out, tt.want) {
				t.Errorf("want %q in %q", tt.want, out)
			}
		})
	}
}

func TestCheckForUpdate(t *testing.T) {
	isolate(t)
	fakeRelease(t, 200, `{"tag_name":"v9.9.9"}`)

	withVersion(t, "dev")
	if out := captureStderr(t, CheckForUpdate); out != "" {
		t.Errorf("dev builds must not nag, got %q", out)
	}
	withVersion(t, "1.0.0") // latest (9.9.9) is now cached from the call above
	if out := captureStderr(t, CheckForUpdate); !strings.Contains(out, "9.9.9") || !strings.Contains(out, "walkline update") {
		t.Errorf("want update hint, got %q", out)
	}
	withVersion(t, "9.9.9")
	if out := captureStderr(t, CheckForUpdate); out != "" {
		t.Errorf("up-to-date must be silent, got %q", out)
	}
}

func TestCheckForUpdate_OfflineIsSilent(t *testing.T) {
	isolate(t)
	withVersion(t, "1.0.0")
	fakeRelease(t, 500, "boom")
	if out := captureStderr(t, CheckForUpdate); out != "" {
		t.Errorf("failed lookup must be silent, got %q", out)
	}
}

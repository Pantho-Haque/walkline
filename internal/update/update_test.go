package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"0.2.0", "0.1.3", true},
		{"0.1.3", "0.1.3", false},
		{"0.1.3", "0.2.0", false}, // never "downgrade" to an older release
		{"1.0.0", "0.9.9", true},
		{"0.10.0", "0.9.0", true}, // numeric, not lexical, comparison
		{"0.1.10", "0.1.9", true},
		{"v0.2.0", "0.1.0", true}, // leading v tolerated
		{"0.2.0", "v0.2.0", false},
		{"0.2.0", "dev", true}, // a dev build is always replaceable by a release
		{"0.2.0", "", true},
		{"0.2.0", "0.2.0-rc1", false}, // pre-release suffix ignored
		{"", "0.1.0", false},          // unknown latest never triggers an update
		{"garbage", "0.1.0", false},
		{"1.2", "0.1.0", false},
	}
	for _, tt := range tests {
		if got := Newer(tt.latest, tt.current); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}

func TestArchiveName(t *testing.T) {
	if got := ArchiveName("1.2.3", "darwin", "arm64"); got != "walkline_1.2.3_darwin_arm64.tar.gz" {
		t.Error(got)
	}
	if got := ArchiveName("1.2.3", "windows", "amd64"); got != "walkline_1.2.3_windows_amd64.zip" {
		t.Error(got)
	}
}

func useServer(t *testing.T, h http.Handler) {
	t.Helper()
	srv := httptest.NewServer(h)
	oldAPI, oldBase := APIURL, DownloadBase
	APIURL, DownloadBase = srv.URL+"/api/latest", srv.URL+"/dl"
	t.Cleanup(func() { srv.Close(); APIURL, DownloadBase = oldAPI, oldBase })
}

func TestLatest(t *testing.T) {
	useServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v0.4.2","name":"x"}`)
	}))
	v, err := Latest()
	if err != nil || v != "0.4.2" {
		t.Fatalf("got %q, %v", v, err)
	}
}

func TestLatestErrors(t *testing.T) {
	for name, h := range map[string]http.HandlerFunc{
		"404":        func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) },
		"bad json":   func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<html>") },
		"no tag":     func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"name":"x"}`) },
		"rate-limit": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403) },
	} {
		t.Run(name, func(t *testing.T) {
			useServer(t, h)
			if v, err := Latest(); err == nil {
				t.Errorf("expected error, got %q", v)
			}
		})
	}
}

func TestCachedFetchesOncePerDay(t *testing.T) {
	var hits int32
	useServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		fmt.Fprint(w, `{"tag_name":"v0.9.0"}`)
	}))
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		v, err := Cached(dir)
		if err != nil || v != "0.9.0" {
			t.Fatalf("call %d: %q, %v", i, v, err)
		}
	}
	if hits != 1 {
		t.Errorf("expected 1 network hit, got %d", hits)
	}

	// An expired check refetches.
	old := time.Now().Add(-25 * time.Hour).Format(time.RFC3339)
	os.WriteFile(filepath.Join(dir, ".last_update_check"), []byte(old), 0o644)
	Cached(dir)
	if hits != 2 {
		t.Errorf("expected refetch after expiry, hits=%d", hits)
	}
}

func TestCachedOfflineIsRememberedAndFallsBack(t *testing.T) {
	var hits int32
	useServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(500)
	}))
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".latest_version"), []byte("0.5.0\n"), 0o644)

	v, err := Cached(dir)
	if err == nil || v != "0.5.0" {
		t.Errorf("first call should error but return last known: %q, %v", v, err)
	}
	v, err = Cached(dir)
	if err != nil || v != "0.5.0" || hits != 1 {
		t.Errorf("failure must be remembered for a day: %q, %v, hits=%d", v, err, hits)
	}
}

// release builds a release server for the current platform serving a fake binary.
func release(t *testing.T, binary string, tamper bool) {
	t.Helper()
	var archive []byte
	name := ArchiveName("9.9.9", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w, _ := zw.Create("walkline.exe")
		w.Write([]byte(binary))
		zw.Close()
		archive = buf.Bytes()
	} else {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		tw.WriteHeader(&tar.Header{Name: "README", Mode: 0o644, Size: 2})
		tw.Write([]byte("hi"))
		tw.WriteHeader(&tar.Header{Name: "walkline", Mode: 0o755, Size: int64(len(binary))})
		tw.Write([]byte(binary))
		tw.Close()
		gz.Close()
		archive = buf.Bytes()
	}
	sum := sha256.Sum256(archive)
	hexsum := hex.EncodeToString(sum[:])
	if tamper {
		hexsum = strings.Repeat("0", 64)
	}
	useServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dl/v9.9.9/" + name:
			w.Write(archive)
		case "/dl/v9.9.9/checksums.txt":
			fmt.Fprintf(w, "%s  %s\n%s  other_file.tar.gz\n", hexsum, name, strings.Repeat("a", 64))
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestInstallReplacesBinary(t *testing.T) {
	release(t, "NEW-BINARY", false)
	target := filepath.Join(t.TempDir(), "walkline")
	os.WriteFile(target, []byte("OLD"), 0o755)

	if err := Install("9.9.9", target); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "NEW-BINARY" {
		t.Errorf("binary not replaced: %q", got)
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(target)
		if st.Mode()&0o111 == 0 {
			t.Error("replaced binary is not executable")
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(target))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".new") {
			t.Errorf("staging file left behind: %s", e.Name())
		}
	}
}

func TestInstallRejectsChecksumMismatchAndKeepsOldBinary(t *testing.T) {
	release(t, "EVIL", true)
	target := filepath.Join(t.TempDir(), "walkline")
	os.WriteFile(target, []byte("OLD"), 0o755)

	err := Install("9.9.9", target)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
	if got, _ := os.ReadFile(target); string(got) != "OLD" {
		t.Errorf("old binary must survive a failed update, got %q", got)
	}
}

func TestInstallMissingRelease(t *testing.T) {
	release(t, "x", false)
	target := filepath.Join(t.TempDir(), "walkline")
	os.WriteFile(target, []byte("OLD"), 0o755)
	if err := Install("1.0.0", target); err == nil {
		t.Fatal("expected error for a version that has no assets")
	}
	if got, _ := os.ReadFile(target); string(got) != "OLD" {
		t.Error("binary modified on failure")
	}
}

func TestInstallIntoMissingDirFails(t *testing.T) {
	release(t, "x", false)
	if err := Install("9.9.9", filepath.Join(t.TempDir(), "nope", "walkline")); err == nil {
		t.Fatal("expected staging error")
	}
}

func TestVerify(t *testing.T) {
	data := []byte("payload")
	sum := sha256.Sum256(data)
	h := hex.EncodeToString(sum[:])
	if err := verify(data, []byte(h+"  a.tar.gz\n"), "a.tar.gz"); err != nil {
		t.Error(err)
	}
	if err := verify(data, []byte(h+" *a.tar.gz\n"), "a.tar.gz"); err != nil {
		t.Errorf("binary-mode marker should be accepted: %v", err)
	}
	if err := verify(data, []byte(h+"  b.tar.gz\n"), "a.tar.gz"); err == nil {
		t.Error("missing entry must fail")
	}
	if err := verify([]byte("tampered"), []byte(h+"  a.tar.gz\n"), "a.tar.gz"); err == nil {
		t.Error("mismatch must fail")
	}
}

func TestExtractErrors(t *testing.T) {
	if _, err := extract([]byte("not gzip"), false); err == nil {
		t.Error("garbage tar.gz")
	}
	if _, err := extract([]byte("not zip"), true); err == nil {
		t.Error("garbage zip")
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "other", Mode: 0o644, Size: 1})
	tw.Write([]byte("x"))
	tw.Close()
	gz.Close()
	if _, err := extract(buf.Bytes(), false); err == nil {
		t.Error("archive without walkline binary must fail")
	}
}

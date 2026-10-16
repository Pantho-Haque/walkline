// Package update checks GitHub releases for a newer walkline and replaces the running binary.
package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Overridable in tests.
var (
	APIURL       = "https://api.github.com/repos/Pantho-Haque/walkline/releases/latest"
	DownloadBase = "https://github.com/Pantho-Haque/walkline/releases/download"
	Client       = &http.Client{Timeout: 60 * time.Second}
)

const checkInterval = 24 * time.Hour

func get(url string) ([]byte, error) {
	resp, err := Client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// Latest returns the newest release version without the leading "v".
func Latest() (string, error) {
	body, err := get(APIURL)
	if err != nil {
		return "", err
	}
	var r struct {
		Tag string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &r); err != nil || r.Tag == "" {
		return "", fmt.Errorf("could not parse latest release")
	}
	return strings.TrimPrefix(r.Tag, "v"), nil
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// Newer reports whether latest is a higher release than current.
// A current version that is not a release number (e.g. "dev") is always older.
func Newer(latest, current string) bool {
	l, ok := parse(latest)
	if !ok {
		return false
	}
	c, ok := parse(current)
	if !ok {
		return true
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// Cached returns the latest version, hitting GitHub at most once per day.
// A failed lookup is also remembered for a day so offline use stays fast.
func Cached(dataDir string) (string, error) {
	checkFile := filepath.Join(dataDir, ".last_update_check")
	versionFile := filepath.Join(dataDir, ".latest_version")
	known, _ := os.ReadFile(versionFile)

	if data, err := os.ReadFile(checkFile); err == nil {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); err == nil && time.Since(t) < checkInterval {
			return strings.TrimSpace(string(known)), nil
		}
	}

	os.MkdirAll(dataDir, 0o755)
	os.WriteFile(checkFile, []byte(time.Now().Format(time.RFC3339)), 0o644)
	latest, err := Latest()
	if err != nil {
		return strings.TrimSpace(string(known)), err
	}
	os.WriteFile(versionFile, []byte(latest), 0o644)
	return latest, nil
}

// ArchiveName is the release asset name for a platform.
func ArchiveName(version, goos, goarch string) string {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("walkline_%s_%s_%s%s", version, goos, goarch, ext)
}

// Install downloads version, verifies its checksum and atomically replaces the binary at target.
func Install(version, target string) error {
	archive := ArchiveName(version, runtime.GOOS, runtime.GOARCH)
	base := fmt.Sprintf("%s/v%s", DownloadBase, version)

	data, err := get(base + "/" + archive)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	sums, err := get(base + "/checksums.txt")
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	if err := verify(data, sums, archive); err != nil {
		return err
	}

	bin, err := extract(data, runtime.GOOS == "windows")
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	// Stage next to the target (same filesystem) so the rename is atomic; the running
	// process keeps its old inode, so replacing the file in place is safe on unix.
	stage := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".new")
	if err := os.WriteFile(stage, bin, 0o755); err != nil {
		return fmt.Errorf("stage new binary: %w", err)
	}
	if runtime.GOOS == "windows" { // a running .exe can be renamed but not overwritten
		old := target + ".old"
		os.Remove(old)
		os.Rename(target, old)
	}
	if err := os.Rename(stage, target); err != nil {
		os.Remove(stage)
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

func verify(data, sums []byte, name string) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	for _, line := range strings.Split(string(sums), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name {
			if f[0] != got {
				return fmt.Errorf("checksum mismatch for %s", name)
			}
			return nil
		}
	}
	return fmt.Errorf("no checksum listed for %s", name)
}

func extract(data []byte, isZip bool) ([]byte, error) {
	if isZip {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if f.Name == "walkline.exe" {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
		return nil, fmt.Errorf("walkline.exe not found in archive")
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("walkline not found in archive")
		}
		if err != nil {
			return nil, err
		}
		if h.Name == "walkline" {
			return io.ReadAll(tr)
		}
	}
}

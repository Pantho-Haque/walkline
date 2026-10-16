package constants

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// The release workflow stamps the version with `-X <pkg>.Version=...`. If that path ever stops
// pointing at the real variable, every release silently ships as "dev" (this already happened once).
func TestReleaseWorkflowStampsRealVersionVariable(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Skip("workflow not available:", err)
	}
	m := regexp.MustCompile(`-X ([\w./-]+)\.Version=`).FindSubmatch(data)
	if m == nil {
		t.Fatal("release.yml does not stamp a Version via -X ldflags")
	}
	if got := string(m[1]); got != "walkline/internal/constants" {
		t.Fatalf("ldflags target %q, but Version is declared in walkline/internal/constants", got)
	}
}

func TestDefaultVersionIsDev(t *testing.T) {
	if Version != "dev" {
		t.Errorf("source default must be \"dev\" (releases override it via ldflags), got %q", Version)
	}
}

func TestPathsLiveUnderHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got, want := DataDir(), filepath.Join(home, ".walkline"); got != want {
		t.Errorf("DataDir = %q, want %q", got, want)
	}
	if got, want := DBPath(), filepath.Join(home, ".walkline", "commits.db"); got != want {
		t.Errorf("DBPath = %q, want %q", got, want)
	}
	if got, want := TemplateDir(), filepath.Join(home, ".git-templates"); got != want {
		t.Errorf("TemplateDir = %q, want %q", got, want)
	}
}

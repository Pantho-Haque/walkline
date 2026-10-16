package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"walkline/internal/store"
)

func TestRunSync(t *testing.T) {
	isolate(t)
	s := openStore(t)

	repo := newRepo(t, "synced")
	addRemote(t, repo)
	pushedH := commit(t, repo, "a", "a")
	git(t, repo, "push", "-q", "-u", "origin", "main")
	localH := commit(t, repo, "b", "b")

	other := newRepo(t, "other")
	otherH := commit(t, other, "a", "a")
	addRemote(t, other)
	git(t, other, "push", "-q", "-u", "origin", "main")

	missing := filepath.Join(t.TempDir(), "gone")
	for _, c := range []store.Commit{
		{Hash: pushedH, ProjectName: "synced", ProjectPath: repo},
		{Hash: localH, ProjectName: "synced", ProjectPath: repo},
		{Hash: otherH, ProjectName: "other", ProjectPath: other},
		{Hash: strings.Repeat("c", 40), ProjectName: "gone", ProjectPath: missing},
	} {
		c.CommittedAt = "2026-01-01T00:00:00Z"
		if err := s.InsertCommit(&c); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("project filter only touches that project", func(t *testing.T) {
		out := captureStdout(t, func() {
			if err := runSync(s, "synced"); err != nil {
				t.Error(err)
			}
		})
		if !strings.Contains(out, "Synced 1") {
			t.Errorf("output %q", out)
		}
		get := func(h string) bool { c, _ := s.GetCommitByHash(h); return c.Pushed }
		if !get(pushedH) || get(localH) || get(otherH) {
			t.Errorf("pushed=%v local=%v other=%v", get(pushedH), get(localH), get(otherH))
		}
	})

	t.Run("all pending projects; missing paths are skipped silently", func(t *testing.T) {
		out := captureStdout(t, func() {
			if err := runSync(s, ""); err != nil {
				t.Error(err)
			}
		})
		if !strings.Contains(out, "Synced") {
			t.Errorf("output %q", out)
		}
		if c, _ := s.GetCommitByHash(otherH); !c.Pushed {
			t.Error("other project should now be pushed")
		}
		if c, _ := s.GetCommitByHash(localH); c.Pushed {
			t.Error("unpushed local commit must remain pending")
		}
	})

	t.Run("sync never deletes commits", func(t *testing.T) {
		// commits on a side branch (not reachable from HEAD) must survive
		git(t, repo, "checkout", "-q", "-b", "side")
		sideH := commit(t, repo, "s", "s")
		s.InsertCommit(&store.Commit{Hash: sideH, ProjectName: "synced", ProjectPath: repo, CommittedAt: "2026-01-03T00:00:00Z"})
		git(t, repo, "checkout", "-q", "main")
		captureStdout(t, func() { runSync(s, "") })
		if c, _ := s.GetCommitByHash(sideH); c == nil {
			t.Error("commit from another branch was deleted")
		}
	})
}

func TestSyncCmd(t *testing.T) {
	isolate(t)
	out, err := run(t, SyncCmd())
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Errorf("empty DB should print nothing, got %q", out)
	}
	if _, err := run(t, SyncCmd(), "--project=nope"); err != nil {
		t.Fatal(err)
	}
}

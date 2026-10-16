package cli

import (
	"strings"
	"testing"

	"walkline/internal/store"
)

func TestGetCommitInfo(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		remote      string
		wantProject string
	}{
		{"dir name when no remote", "my-local-repo", "", "my-local-repo"},
		{"https remote", "ignored1", "https://github.com/acme/widgets.git", "widgets"},
		{"ssh remote", "ignored2", "git@github.com:acme/gadgets.git", "gadgets"},
		{"remote without .git", "ignored3", "https://example.com/acme/thing", "thing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolate(t)
			repo := newRepo(t, tt.dir)
			if tt.remote != "" {
				git(t, repo, "remote", "add", "origin", tt.remote)
			}
			want := commit(t, repo, "f.txt", "feat: héllo \"world\"\n\nbody line")

			c, err := getCommitInfo(repo)
			if err != nil {
				t.Fatal(err)
			}
			if c.Hash != want {
				t.Errorf("hash %s want %s", c.Hash, want)
			}
			if c.ProjectName != tt.wantProject {
				t.Errorf("project %q want %q", c.ProjectName, tt.wantProject)
			}
			if c.ProjectPath != repo {
				t.Errorf("path %q want %q", c.ProjectPath, repo)
			}
			if c.RemoteURL != tt.remote {
				t.Errorf("remote %q want %q", c.RemoteURL, tt.remote)
			}
			if c.AuthorName != "Tester" || c.AuthorEmail != "tester@example.com" {
				t.Errorf("author %q <%q>", c.AuthorName, c.AuthorEmail)
			}
			if !strings.HasPrefix(c.Message, "feat: héllo \"world\"") || !strings.Contains(c.Message, "body line") {
				t.Errorf("message not preserved: %q", c.Message)
			}
			if len(c.CommittedAt) < 20 || c.CommittedAt[4] != '-' || c.CommittedAt[10] != 'T' {
				t.Errorf("committed_at not ISO-8601: %q", c.CommittedAt)
			}
		})
	}
}

func TestGetCommitInfo_Errors(t *testing.T) {
	isolate(t)
	if _, err := getCommitInfo(t.TempDir()); err == nil {
		t.Error("expected error outside a repo")
	}
	empty := newRepo(t, "empty")
	if _, err := getCommitInfo(empty); err == nil {
		t.Error("expected error for repo with no commits")
	}
}

func TestLogCommitCmd(t *testing.T) {
	isolate(t)
	repo := newRepo(t, "proj")
	h1 := commit(t, repo, "a", "one")
	chdir(t, repo)

	if _, err := run(t, LogCommitCmd()); err != nil {
		t.Fatal(err)
	}
	// logging the same HEAD twice stays a single row (hook may fire repeatedly)
	if _, err := run(t, LogCommitCmd()); err != nil {
		t.Fatal(err)
	}
	h2 := commit(t, repo, "b", "two")
	if _, err := run(t, LogCommitCmd()); err != nil {
		t.Fatal(err)
	}

	s := openStore(t)
	cs, err := s.QueryCommits(store.CommitFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Fatalf("want 2 rows, got %d", len(cs))
	}
	got := map[string]store.Commit{cs[0].Hash: cs[0], cs[1].Hash: cs[1]}
	for _, h := range []string{h1, h2} {
		c, ok := got[h]
		if !ok {
			t.Fatalf("missing %s", h)
		}
		if c.Pushed || c.ProjectName != "proj" || c.ProjectPath != repo {
			t.Errorf("bad row %+v", c)
		}
	}
}

func TestLogCommitCmd_NotARepo(t *testing.T) {
	isolate(t)
	chdir(t, t.TempDir())
	if _, err := run(t, LogCommitCmd()); err == nil {
		t.Error("expected error outside a repo")
	}
}

func TestMarkPushedCmd(t *testing.T) {
	setup := func(t *testing.T) (repo string, hs []string) {
		isolate(t)
		repo = newRepo(t, "proj")
		for _, f := range []string{"a", "b", "c"} {
			hs = append(hs, commit(t, repo, f, f))
		}
		chdir(t, repo)
		s := openStore(t)
		for _, h := range hs {
			s.InsertCommit(&store.Commit{Hash: h, ProjectName: "proj", ProjectPath: repo, CommittedAt: "2026-01-01T00:00:00Z"})
		}
		return
	}

	t.Run("--range marks only the range", func(t *testing.T) {
		_, hs := setup(t)
		if _, err := run(t, MarkPushedCmd(), "--range", hs[0]+".."+hs[2], "--remote-ref", "refs/heads/main"); err != nil {
			t.Fatal(err)
		}
		for i, want := range []bool{false, true, true} {
			if got := pushedState(t, hs[i]); got != want {
				t.Errorf("commit %d pushed=%v want %v", i, got, want)
			}
		}
	})

	t.Run("positional ref", func(t *testing.T) {
		_, hs := setup(t)
		if _, err := run(t, MarkPushedCmd(), hs[1]+"..HEAD"); err != nil {
			t.Fatal(err)
		}
		if pushedState(t, hs[1]) || !pushedState(t, hs[2]) {
			t.Error("only the commit after hs[1] should be marked")
		}
	})

	t.Run("no upstream marks everything reachable from HEAD", func(t *testing.T) {
		_, hs := setup(t)
		if _, err := run(t, MarkPushedCmd()); err != nil {
			t.Fatal(err)
		}
		for i, h := range hs {
			if !pushedState(t, h) {
				t.Errorf("commit %d should be pushed", i)
			}
		}
	})

	t.Run("auto-detect uses upstream..HEAD", func(t *testing.T) {
		repo, hs := setup(t)
		addRemote(t, repo)
		git(t, repo, "push", "-q", "-u", "origin", "main")
		h4 := commit(t, repo, "d", "d")
		openStore(t).InsertCommit(&store.Commit{Hash: h4, ProjectName: "proj", ProjectPath: repo, CommittedAt: "2026-01-02T00:00:00Z"})
		if _, err := run(t, MarkPushedCmd()); err != nil {
			t.Fatal(err)
		}
		if !pushedState(t, h4) {
			t.Error("commit ahead of upstream should be marked")
		}
		if pushedState(t, hs[0]) {
			t.Error("commits already in upstream are outside upstream..HEAD, so untouched")
		}
	})

	t.Run("empty range is a no-op", func(t *testing.T) {
		_, hs := setup(t)
		if _, err := run(t, MarkPushedCmd(), "--range", "HEAD..HEAD"); err != nil {
			t.Fatal(err)
		}
		if pushedState(t, hs[2]) {
			t.Error("nothing should change")
		}
	})

	t.Run("bad range errors", func(t *testing.T) {
		setup(t)
		if _, err := run(t, MarkPushedCmd(), "--range", "nope..alsonope"); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("too many args", func(t *testing.T) {
		setup(t)
		if _, err := run(t, MarkPushedCmd(), "a", "b"); err == nil {
			t.Error("expected arg error")
		}
	})
}

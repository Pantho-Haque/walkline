package store

import (
	"path/filepath"
	"reflect"
	"testing"
)

func seed(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	rows := []Commit{
		{Hash: "aaa1111", ProjectName: "alpha", ProjectPath: "/p/alpha", AuthorName: "Ann Lee", AuthorEmail: "ann@x.io", Message: "Fix login bug", CommittedAt: "2026-10-14T09:00:00+06:00"},
		{Hash: "bbb2222", ProjectName: "alpha-ui", ProjectPath: "/p/alpha-ui", AuthorName: "Bob", AuthorEmail: "bob@y.io", Message: "Reach 100% coverage", CommittedAt: "2026-10-15T10:00:00+06:00"},
		{Hash: "ccc3333", ProjectName: "Beta", ProjectPath: "/p/beta", AuthorName: "ann", AuthorEmail: "ann@z.io", Message: "snake_case rename", CommittedAt: "2026-10-16T23:30:00+06:00"},
		{Hash: "ddd4444", ProjectName: "gamma", ProjectPath: "/p/gamma", AuthorName: "", AuthorEmail: "", Message: "multi\nline body", CommittedAt: "2026-10-16T08:00:00+06:00"},
	}
	for i := range rows {
		if err := s.InsertCommit(&rows[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.MarkPushed([]string{"aaa1111", "ccc3333"}); err != nil {
		t.Fatal(err)
	}
	return s
}

func hashes(cs []Commit) []string {
	out := []string{}
	for _, c := range cs {
		out = append(out, c.Hash)
	}
	return out
}

func TestQueryCommitsFilters(t *testing.T) {
	s := seed(t)
	yes, no := true, false
	tests := []struct {
		name string
		f    CommitFilter
		want []string
	}{
		{"no filter, newest first", CommitFilter{}, []string{"ccc3333", "ddd4444", "bbb2222", "aaa1111"}},
		{"text matches message case-insensitively", CommitFilter{Text: "LOGIN"}, []string{"aaa1111"}},
		{"text matches hash prefix", CommitFilter{Text: "bbb22"}, []string{"bbb2222"}},
		{"text matches project", CommitFilter{Text: "gamma"}, []string{"ddd4444"}},
		{"text matches author name", CommitFilter{Text: "bob"}, []string{"bbb2222"}},
		{"text matches author email", CommitFilter{Text: "z.io"}, []string{"ccc3333"}},
		{"text matches multi-line body", CommitFilter{Text: "line body"}, []string{"ddd4444"}},
		{"percent is literal, not a wildcard", CommitFilter{Text: "100%"}, []string{"bbb2222"}},
		{"lone percent only matches literal percent", CommitFilter{Text: "%"}, []string{"bbb2222"}},
		{"underscore is literal", CommitFilter{Text: "snake_case"}, []string{"ccc3333"}},
		{"underscore does not act as single-char wildcard", CommitFilter{Text: "snake-case"}, []string{}},
		{"project exact", CommitFilter{Project: "alpha"}, []string{"aaa1111"}},
		{"project contains is substring + case-insensitive", CommitFilter{ProjectContains: "ALPH"}, []string{"bbb2222", "aaa1111"}},
		{"author partial, case-insensitive across name+email", CommitFilter{Author: "ANN"}, []string{"ccc3333", "aaa1111"}},
		{"pushed only", CommitFilter{Pushed: &yes}, []string{"ccc3333", "aaa1111"}},
		{"pending only", CommitFilter{Pushed: &no}, []string{"ddd4444", "bbb2222"}},
		{"filters are ANDed", CommitFilter{Author: "ann", Pushed: &yes, ProjectContains: "beta"}, []string{"ccc3333"}},
		{"limit", CommitFilter{Limit: 2}, []string{"ccc3333", "ddd4444"}},
		{"since", CommitFilter{Since: "2026-10-16T00:00:00Z"}, []string{"ccc3333", "ddd4444"}},
		{"until", CommitFilter{Until: "2026-10-14T23:59:59Z"}, []string{"aaa1111"}},
		{"no match", CommitFilter{Text: "zzz-nothing"}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.QueryCommits(tt.f)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(hashes(got), tt.want) {
				t.Errorf("got %v, want %v", hashes(got), tt.want)
			}
		})
	}
}

func TestQueryCommitsRoundTripsFields(t *testing.T) {
	s := seed(t)
	got, _ := s.QueryCommits(CommitFilter{Text: "aaa1111"})
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	c := got[0]
	if !c.Pushed || c.PushedAt == "" || c.AuthorEmail != "ann@x.io" || c.ProjectPath != "/p/alpha" {
		t.Errorf("fields lost: %+v", c)
	}
}

func TestStats(t *testing.T) {
	s := seed(t)
	st, err := s.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if st != (Stats{Total: 4, Pending: 2, Projects: 4}) {
		t.Errorf("got %+v", st)
	}
}

func TestStatsEmptyDB(t *testing.T) {
	s, _ := NewStore(filepath.Join(t.TempDir(), "e.db"))
	defer s.Close()
	st, err := s.Stats()
	if err != nil || st != (Stats{}) {
		t.Errorf("got %+v, %v", st, err)
	}
}

func TestDistinct(t *testing.T) {
	s := seed(t)
	projects, _ := s.DistinctProjects()
	if want := []string{"alpha", "alpha-ui", "Beta", "gamma"}; !reflect.DeepEqual(projects, want) {
		t.Errorf("projects: got %v, want %v", projects, want)
	}
	authors, _ := s.DistinctAuthors()
	if want := []string{"ann", "Ann Lee", "Bob"}; !reflect.DeepEqual(authors, want) {
		t.Errorf("authors (empty names excluded): got %v, want %v", authors, want)
	}
}

func TestCommitsAreNeverDeleted(t *testing.T) {
	// walkline's contract: every commit ever recorded stays recorded.
	s := seed(t)
	s.MarkPushed([]string{"bbb2222"})
	s.InsertCommit(&Commit{Hash: "aaa1111", ProjectName: "changed", CommittedAt: "2000-01-01T00:00:00Z"})
	all, _ := s.QueryCommits(CommitFilter{})
	if len(all) != 4 {
		t.Fatalf("expected 4 commits retained, got %d", len(all))
	}
}

package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"walkline/internal/store"
)

var nasty = []store.Commit{
	{Hash: "aaaaaaa1111111111111111111111111111111aa", ProjectName: "p1", ProjectPath: "/a/p1", AuthorName: "Ana \"Q\" Ünï", AuthorEmail: "ana@x.io",
		Message: "fix: \"quotes\", commas,\nnewline\ttab \\ backslash 日本語 🚀", CommittedAt: "2026-03-01T10:00:00+06:00", Pushed: true, PushedAt: "2026-03-01T11:00:00Z"},
	{Hash: "bbbbbbb2222222222222222222222222222222bb", ProjectName: "p2", ProjectPath: "/b/p2", AuthorName: "Bob", AuthorEmail: "bob@x.io",
		Message: "plain", CommittedAt: "2026-03-02T10:00:00+06:00"},
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, nasty); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(got) != 2 {
		t.Fatalf("len %d", len(got))
	}
	if got[0]["message"] != nasty[0].Message {
		t.Errorf("message round-trip: %q", got[0]["message"])
	}
	if got[0]["author"] != nasty[0].AuthorName || got[0]["hash"] != nasty[0].Hash || got[0]["pushed"] != true {
		t.Errorf("fields: %v", got[0])
	}
	if _, has := got[1]["pushed_at"]; has {
		t.Error("pushed_at should be omitted for pending commits")
	}

	buf.Reset()
	if err := WriteJSON(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Errorf("empty export should be [], got %q", buf.String())
	}
}

func TestWriteCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCSV(&buf, nasty); err != nil {
		t.Fatal(err)
	}
	recs, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("invalid CSV: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("rows %d", len(recs))
	}
	if recs[0][0] != "hash" || len(recs[0]) != 9 {
		t.Errorf("header %v", recs[0])
	}
	if recs[1][5] != nasty[0].Message || recs[1][3] != nasty[0].AuthorName {
		t.Errorf("round-trip failed: %q / %q", recs[1][5], recs[1][3])
	}
	if recs[1][7] != "1" || recs[2][7] != "0" || recs[1][8] != nasty[0].PushedAt || recs[2][8] != "" {
		t.Errorf("pushed columns: %v %v", recs[1], recs[2])
	}
}

func TestWriteReport(t *testing.T) {
	var buf bytes.Buffer
	writeReport(&buf, nasty[:1])
	writeReport(&buf, nasty[1:])
	out := buf.String()
	for _, want := range []string{"● p1 (aaaaaaa) • 2026-03-01 • [✓ Pushed]", "Author: Ana \"Q\" Ünï <ana@x.io>", "● p2 (bbbbbbb) • 2026-03-02 • [✗ Pending]", "Message: plain"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	buf.Reset()
	writeReport(&buf, nil)
	if buf.Len() != 0 {
		t.Error("no commits -> no output")
	}
}

func TestFilterFlagsBuild(t *testing.T) {
	tr, fa := true, false
	tests := []struct {
		name       string
		f          filterFlags
		wantErr    bool
		wantPushed *bool
		check      func(t *testing.T, got store.CommitFilter)
	}{
		{"empty", filterFlags{}, false, nil, nil},
		{"pushed only", filterFlags{pushed: true}, false, &tr, nil},
		{"pending only", filterFlags{pending: true}, false, &fa, nil},
		{"both flags = no status filter", filterFlags{pushed: true, pending: true}, false, nil, nil},
		{"passes project/author/limit", filterFlags{project: "p", author: "a", limit: 7}, false, nil, func(t *testing.T, g store.CommitFilter) {
			if g.Project != "p" || g.Author != "a" || g.Limit != 7 {
				t.Errorf("%+v", g)
			}
		}},
		{"since", filterFlags{since: "2026-01-02"}, false, nil, func(t *testing.T, g store.CommitFilter) {
			if g.Since != "2026-01-02T00:00:00Z" || g.Until != "" {
				t.Errorf("%+v", g)
			}
		}},
		{"at", filterFlags{at: "2026-01-02"}, false, nil, func(t *testing.T, g store.CommitFilter) {
			if g.Since != "2026-01-02T00:00:00Z" || g.Until != "2026-01-02T23:59:59Z" {
				t.Errorf("%+v", g)
			}
		}},
		{"mutually exclusive dates", filterFlags{since: "2026-01-01", until: "2026-01-02"}, true, nil, nil},
		{"garbage date", filterFlags{since: "abcd-ef-gh"}, true, nil, nil},
		{"impossible date", filterFlags{until: "2026-02-30"}, true, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.build()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v", err)
			}
			if tt.wantErr {
				return
			}
			switch {
			case tt.wantPushed == nil && got.Pushed != nil:
				t.Errorf("Pushed should be nil, got %v", *got.Pushed)
			case tt.wantPushed != nil && (got.Pushed == nil || *got.Pushed != *tt.wantPushed):
				t.Errorf("Pushed mismatch: %v", got.Pushed)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// seed fills the temp-HOME DB with commits whose project paths don't exist (so sync is a no-op).
func seed(t *testing.T) {
	t.Helper()
	s := openStore(t)
	for _, c := range []store.Commit{
		{Hash: "1111111aaaa", ProjectName: "alpha", ProjectPath: "/nope/alpha", AuthorName: "Ann", AuthorEmail: "ann@x.io", Message: "alpha one", CommittedAt: "2026-03-01T10:00:00+06:00"},
		{Hash: "2222222bbbb", ProjectName: "alpha", ProjectPath: "/nope/alpha", AuthorName: "Bob", AuthorEmail: "bob@x.io", Message: "alpha \"two\"\nmulti", CommittedAt: "2026-03-05T10:00:00+06:00"},
		{Hash: "3333333cccc", ProjectName: "beta", ProjectPath: "/nope/beta", AuthorName: "Ann", AuthorEmail: "ann@x.io", Message: "beta one", CommittedAt: "2026-03-09T10:00:00+06:00"},
	} {
		if err := s.InsertCommit(&c); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.MarkPushed([]string{"2222222bbbb"}); err != nil {
		t.Fatal(err)
	}
}

func TestReportCmd(t *testing.T) {
	isolate(t)
	seed(t)
	tests := []struct {
		name      string
		args      []string
		want      []string
		notWanted []string
	}{
		{"all, newest first", nil, []string{"alpha one", "beta one"}, nil},
		{"project", []string{"--project=beta"}, []string{"beta one"}, []string{"alpha"}},
		{"author partial", []string{"--author=bo"}, []string{"alpha \"two\""}, []string{"beta one", "alpha one"}},
		{"pushed", []string{"--pushed"}, []string{"[✓ Pushed]"}, []string{"[✗ Pending]"}},
		{"pending", []string{"--pending"}, []string{"alpha one", "beta one"}, []string{"[✓ Pushed]"}},
		{"since", []string{"--since=2026-03-05"}, []string{"beta one", "alpha \"two\""}, []string{"alpha one"}},
		{"until", []string{"--until=2026-03-05"}, []string{"alpha one", "alpha \"two\""}, []string{"beta one"}},
		{"at", []string{"--at=2026-03-09"}, []string{"beta one"}, []string{"alpha"}},
		{"limit", []string{"-n", "1"}, []string{"beta one"}, []string{"alpha"}},
		{"no matches", []string{"--project=zzz"}, nil, []string{"alpha", "beta"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := run(t, ReportCmd(), tt.args...)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in\n%s", w, out)
				}
			}
			for _, w := range tt.notWanted {
				if strings.Contains(out, w) {
					t.Errorf("unexpected %q in\n%s", w, out)
				}
			}
		})
	}

	if _, err := run(t, ReportCmd(), "--since=2026-03-01", "--until=2026-03-02"); err == nil {
		t.Error("expected mutually-exclusive date error")
	}
	if _, err := run(t, ReportCmd(), "--since=not-a-date"); err == nil {
		t.Error("expected invalid date error")
	}
}

func TestReportCmd_ReconcilesBeforeReporting(t *testing.T) {
	isolate(t)
	repo := newRepo(t, "live")
	addRemote(t, repo)
	h := commit(t, repo, "a", "pushed already")
	git(t, repo, "push", "-q", "-u", "origin", "main")
	openStore(t).InsertCommit(&store.Commit{Hash: h, ProjectName: "live", ProjectPath: repo, Message: "pushed already", CommittedAt: "2026-03-01T10:00:00Z"})

	out, err := run(t, ReportCmd(), "--pushed")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "pushed already") {
		t.Errorf("report should sync first and show it as pushed:\n%s", out)
	}
}

func TestExportCmd(t *testing.T) {
	isolate(t)
	seed(t)
	dir := t.TempDir()

	t.Run("json valid and filtered", func(t *testing.T) {
		p := filepath.Join(dir, "o.json")
		if _, err := run(t, ExportCmd(), "--format=json", "--out="+p, "--project=alpha"); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(p)
		var got []map[string]any
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("invalid json: %v\n%s", err, b)
		}
		if len(got) != 2 {
			t.Fatalf("want 2, got %d", len(got))
		}
		if got[0]["message"] != "alpha \"two\"\nmulti" {
			t.Errorf("message mangled: %q", got[0]["message"])
		}
	})

	t.Run("csv default format", func(t *testing.T) {
		p := filepath.Join(dir, "o.csv")
		if _, err := run(t, ExportCmd(), "--out="+p, "--pending"); err != nil {
			t.Fatal(err)
		}
		f, _ := os.Open(p)
		defer f.Close()
		recs, err := csv.NewReader(f).ReadAll()
		if err != nil || len(recs) != 3 { // header + 2 pending
			t.Fatalf("recs=%d err=%v", len(recs), err)
		}
	})

	t.Run("bad format rejected before writing", func(t *testing.T) {
		p := filepath.Join(dir, "bad.xml")
		if _, err := run(t, ExportCmd(), "--format=xml", "--out="+p); err == nil {
			t.Fatal("expected error")
		}
		if _, err := os.Stat(p); err == nil {
			t.Error("file must not be created for a bad format")
		}
	})

	t.Run("--out is required", func(t *testing.T) {
		if _, err := run(t, ExportCmd()); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("unwritable path", func(t *testing.T) {
		if _, err := run(t, ExportCmd(), "--out="+filepath.Join(dir, "no", "such", "dir", "x.csv")); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("bad date", func(t *testing.T) {
		if _, err := run(t, ExportCmd(), "--out="+filepath.Join(dir, "d.csv"), "--at=zzz"); err == nil {
			t.Error("expected error")
		}
	})
}

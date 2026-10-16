package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"walkline/internal/store"
)

// filterFlags are the commit filters shared by report and export.
type filterFlags struct {
	since, until, at, project, author string
	pushed, pending                   bool
	limit                             int
}

func (f *filterFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.since, "since", "", "Commits from this date onward (inclusive)")
	cmd.Flags().StringVar(&f.until, "until", "", "Commits up to this date (inclusive)")
	cmd.Flags().StringVar(&f.at, "at", "", "Commits on this exact date only")
	cmd.Flags().StringVar(&f.project, "project", "", "Project name")
	cmd.Flags().StringVar(&f.author, "author", "", "Author name or email (partial match)")
	cmd.Flags().BoolVar(&f.pushed, "pushed", false, "Only pushed commits")
	cmd.Flags().BoolVar(&f.pending, "pending", false, "Only pending (unpushed) commits")
	cmd.Flags().IntVarP(&f.limit, "limit", "n", 0, "Limit number of results")
}

func (f *filterFlags) build() (store.CommitFilter, error) {
	flt, err := store.BuildDateFilter(f.since, f.until, f.at)
	if err != nil {
		return flt, err
	}
	flt.Project, flt.Author, flt.Limit = f.project, f.author, f.limit
	if f.pushed != f.pending {
		v := f.pushed
		flt.Pushed = &v
	}
	return flt, nil
}

func queryFiltered(f *filterFlags) ([]store.Commit, error) {
	s, err := store.New()
	if err != nil {
		return nil, err
	}
	defer s.Close()
	runSync(s, f.project)
	flt, err := f.build()
	if err != nil {
		return nil, err
	}
	return s.QueryCommits(flt)
}

func ReportCmd() *cobra.Command {
	var f filterFlags
	cmd := &cobra.Command{
		Use:     "report",
		Short:   "Show push status report",
		Example: "walkline report --project=myrepo --author=john",
		RunE: func(cmd *cobra.Command, args []string) error {
			commits, err := queryFiltered(&f)
			if err != nil {
				return err
			}
			writeReport(cmd.OutOrStdout(), commits)
			return nil
		},
	}
	f.bind(cmd)
	return cmd
}

func writeReport(w io.Writer, commits []store.Commit) {
	for _, c := range commits {
		status := "[✗ Pending]"
		if c.Pushed {
			status = "[✓ Pushed]"
		}
		fmt.Fprintf(w, "● %s (%.7s) • %.10s • %s\n", c.ProjectName, c.Hash, c.CommittedAt, status)
		fmt.Fprintf(w, "├─ Author: %s <%s>\n", c.AuthorName, c.AuthorEmail)
		fmt.Fprintf(w, "└─ Message: %s\n\n", c.Message)
	}
}

func ExportCmd() *cobra.Command {
	var f filterFlags
	var format, outPath string
	cmd := &cobra.Command{
		Use:     "export",
		Short:   "Export commits to file",
		Example: "walkline export --format=json --out=commits.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "json" && format != "csv" {
				return fmt.Errorf("--format must be json or csv")
			}
			commits, err := queryFiltered(&f)
			if err != nil {
				return err
			}
			out, err := os.Create(outPath)
			if err != nil {
				return err
			}
			defer out.Close()
			if format == "json" {
				return WriteJSON(out, commits)
			}
			return WriteCSV(out, commits)
		},
	}
	cmd.Flags().StringVar(&format, "format", "csv", "Format: json or csv")
	cmd.Flags().StringVar(&outPath, "out", "", "Output file path")
	cmd.MarkFlagRequired("out")
	f.bind(cmd)
	return cmd
}

// WriteJSON writes commits as a JSON array.
func WriteJSON(w io.Writer, commits []store.Commit) error {
	type row struct {
		Hash      string `json:"hash"`
		Project   string `json:"project"`
		Path      string `json:"path"`
		Author    string `json:"author"`
		Email     string `json:"email"`
		Message   string `json:"message"`
		Committed string `json:"committed"`
		Pushed    bool   `json:"pushed"`
		PushedAt  string `json:"pushed_at,omitempty"`
	}
	rows := make([]row, 0, len(commits))
	for _, c := range commits {
		rows = append(rows, row{c.Hash, c.ProjectName, c.ProjectPath, c.AuthorName, c.AuthorEmail, c.Message, c.CommittedAt, c.Pushed, c.PushedAt})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// WriteCSV writes commits as CSV with a header row.
func WriteCSV(w io.Writer, commits []store.Commit) error {
	cw := csv.NewWriter(w)
	cw.Write([]string{"hash", "project_name", "project_path", "author_name", "author_email", "message", "committed_at", "pushed", "pushed_at"})
	for _, c := range commits {
		pushed := "0"
		if c.Pushed {
			pushed = "1"
		}
		cw.Write([]string{c.Hash, c.ProjectName, c.ProjectPath, c.AuthorName, c.AuthorEmail, c.Message, c.CommittedAt, pushed, c.PushedAt})
	}
	cw.Flush()
	return cw.Error()
}

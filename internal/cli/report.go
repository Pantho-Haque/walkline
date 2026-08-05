package cli

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"walkline/internal/store"
)

func ReportCmd() *cobra.Command {
	var since string
	var project string
	var author string
	var pushed, pending bool
	var limit int

	cmd := &cobra.Command{
		Use:     "report",
		Short:   "Show push status report",
		Example: "walkline report --project=myrepo --author=john",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.New()
			if err != nil {
				return err
			}
			defer s.Close()

			runSync(s, project)

			filter := store.CommitFilter{Since: since, Project: project, Author: author, Limit: limit}
			if pushed && !pending {
				v := true
				filter.Pushed = &v
			} else if pending && !pushed {
				v := false
				filter.Pushed = &v
			}

			commits, err := s.QueryCommits(filter)
			if err != nil {
				return err
			}

			for _, c := range commits {
				shortHash := c.Hash[:7]
				date := c.CommittedAt[:10]
				authorLine := fmt.Sprintf("%s <%s>", c.AuthorName, c.AuthorEmail)

				pushedStatus := "[✗ Pending]"
				if c.Pushed {
					pushedStatus = "[✓ Pushed]"
				}

				fmt.Printf("● %s (%s) • %s • %s\n", c.ProjectName, shortHash, date, pushedStatus)
				fmt.Printf("├─ Author: %s\n", authorLine)
				fmt.Printf("└─ Message: %s\n\n", c.Message)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Since date (RFC3339)")
	cmd.Flags().StringVar(&project, "project", "", "Project name")
	cmd.Flags().StringVar(&author, "author", "", "Author name or email (partial match)")
	cmd.Flags().BoolVar(&pushed, "pushed", false, "Only pushed commits")
	cmd.Flags().BoolVar(&pending, "pending", false, "Only pending commits")
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "Limit number of results")
	return cmd
}

func ExportCmd() *cobra.Command {
	var format string
	var outPath string
	var since string
	var project string
	var author string
	var pushed, pending bool

	cmd := &cobra.Command{
		Use:     "export",
		Short:   "Export commits to file",
		Example: "walkline export --format=json --out=commits.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.New()
			if err != nil {
				return err
			}
			defer s.Close()

			runSync(s, project)

			filter := store.CommitFilter{Since: since, Project: project, Author: author}
			if pushed && !pending {
				v := true
				filter.Pushed = &v
			} else if pending && !pushed {
				v := false
				filter.Pushed = &v
			}

			commits, err := s.QueryCommits(filter)
			if err != nil {
				return err
			}

			f, err := os.Create(outPath)
			if err != nil {
				return err
			}
			defer f.Close()

			if format == "json" {
				return exportJSON(f, commits)
			}
			return exportCSV(f, commits)
		},
	}
	cmd.Flags().StringVar(&format, "format", "csv", "Format: json or csv")
	cmd.Flags().StringVar(&outPath, "out", "", "Output file path")
	cmd.MarkFlagRequired("out")
	cmd.Flags().StringVar(&since, "since", "", "Since date")
	cmd.Flags().StringVar(&project, "project", "", "Project name")
	cmd.Flags().StringVar(&author, "author", "", "Author name or email")
	cmd.Flags().BoolVar(&pushed, "pushed", false, "Only pushed")
	cmd.Flags().BoolVar(&pending, "pending", false, "Only pending")
	return cmd
}

func exportJSON(f *os.File, commits []store.Commit) error {
	fmt.Fprintln(f, "[")
	for i, c := range commits {
		comma := ","
		if i == len(commits)-1 {
			comma = ""
		}
		fmt.Fprintf(f, `  {"hash":"%s","project":"%s","path":"%s","author":"%s","message":"%s","committed":"%s","pushed":%t}%s`+"\n",
			c.Hash, c.ProjectName, c.ProjectPath, c.AuthorName, c.Message, c.CommittedAt, c.Pushed, comma)
	}
	fmt.Fprintln(f, "]")
	return nil
}

func exportCSV(f *os.File, commits []store.Commit) error {
	w := csv.NewWriter(f)
	w.Write([]string{"hash", "project_name", "project_path", "author_name", "author_email", "message", "committed_at", "pushed", "pushed_at"})
	for _, c := range commits {
		pushed := "0"
		if c.Pushed {
			pushed = "1"
		}
		w.Write([]string{c.Hash, c.ProjectName, c.ProjectPath, c.AuthorName, c.AuthorEmail, c.Message, c.CommittedAt, pushed, c.PushedAt})
	}
	w.Flush()
	return w.Error()
}

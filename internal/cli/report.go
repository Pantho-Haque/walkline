package cli

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"walkline/internal/store"
)

func ReportCmd() *cobra.Command {
	var since string
	var project string
	var pushed, unpushed bool

	cmd := &cobra.Command{
		Use:     "report",
		Short:   "Show push status report",
		Example: "walkline report --project=myrepo --unpushed",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.New()
			if err != nil {
				return err
			}
			defer s.Close()

			filter := store.CommitFilter{Since: since, Project: project}
			if pushed && !unpushed {
				v := true
				filter.Pushed = &v
			} else if unpushed && !pushed {
				v := false
				filter.Pushed = &v
			}

			commits, err := s.QueryCommits(filter)
			if err != nil {
				return err
			}

			fmt.Printf("%-20s %-12s %-8s %-15s %-40s %s\n",
				"PROJECT", "DATE", "HASH", "AUTHOR", "MESSAGE", "PUSHED")
			fmt.Println(strings.Repeat("-", 110))
			for _, c := range commits {
				shortHash := c.Hash[:7]
				date := c.CommittedAt[:10]
				msg := c.Message
				if len(msg) > 38 {
					msg = msg[:38] + "..."
				}
				pushedStr := "N"
				if c.Pushed {
					pushedStr = "Y"
				}
				fmt.Printf("%-20s %-12s %-8s %-15s %-40s %s\n",
					truncate(c.ProjectName, 20), date, shortHash,
					truncate(c.AuthorName, 15), msg, pushedStr)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Since date (RFC3339)")
	cmd.Flags().StringVar(&project, "project", "", "Project name")
	cmd.Flags().BoolVar(&pushed, "pushed", false, "Only pushed")
	cmd.Flags().BoolVar(&unpushed, "unpushed", false, "Only unpushed")
	return cmd
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func ExportCmd() *cobra.Command {
	var format string
	var outPath string
	var since string
	var project string
	var pushed, unpushed bool

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

			filter := store.CommitFilter{Since: since, Project: project}
			if pushed && !unpushed {
				v := true
				filter.Pushed = &v
			} else if unpushed && !pushed {
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
	cmd.Flags().BoolVar(&pushed, "pushed", false, "Only pushed")
	cmd.Flags().BoolVar(&unpushed, "unpushed", false, "Only unpushed")
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

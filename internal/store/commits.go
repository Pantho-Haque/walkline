package store

import (
	"database/sql"
	"strings"
	"time"
)

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

type Commit struct {
	Hash        string
	ProjectName string
	ProjectPath string
	RemoteURL   string
	AuthorName  string
	AuthorEmail string
	Message     string
	CommittedAt string
	Pushed      bool
	PushedAt    string
}

func (s *Store) InsertCommit(c *Commit) error {
	query := `
	INSERT OR IGNORE INTO commits (hash, project_name, project_path, remote_url, author_name, author_email, message, committed_at, pushed)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)
	`
	_, err := s.db.Exec(query, c.Hash, c.ProjectName, c.ProjectPath, c.RemoteURL, c.AuthorName, c.AuthorEmail, c.Message, c.CommittedAt)
	return err
}

func (s *Store) MarkPushed(hashes []string) error {
	if len(hashes) == 0 {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("UPDATE commits SET pushed = 1, pushed_at = ? WHERE hash = ?")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, h := range hashes {
		_, err := stmt.Exec(now, h)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

type CommitFilter struct {
	Since   string
	Until   string
	At      string
	Project string
	// ProjectContains is a case-insensitive substring match on project name.
	ProjectContains string
	Author          string
	Pushed          *bool
	Limit           int
	// Text is a case-insensitive substring match on message, hash, project, or author.
	Text string
}

func (s *Store) QueryCommits(f CommitFilter) ([]Commit, error) {
	query := "SELECT hash, project_name, project_path, remote_url, author_name, author_email, message, committed_at, pushed, pushed_at FROM commits WHERE 1=1"
	var args []interface{}
	if f.Since != "" {
		query += " AND committed_at >= ?"
		args = append(args, f.Since)
	}
	if f.Until != "" {
		query += " AND committed_at <= ?"
		args = append(args, f.Until)
	}
	if f.Project != "" {
		query += " AND project_name = ?"
		args = append(args, f.Project)
	}
	if f.ProjectContains != "" {
		query += " AND project_name LIKE ? ESCAPE '\\'"
		args = append(args, "%"+likeEscaper.Replace(f.ProjectContains)+"%")
	}
	if f.Author != "" {
		query += " AND (author_name LIKE ? ESCAPE '\\' OR author_email LIKE ? ESCAPE '\\')"
		likeAuthor := "%" + likeEscaper.Replace(f.Author) + "%"
		args = append(args, likeAuthor, likeAuthor)
	}
	if f.Text != "" {
		query += " AND (message LIKE ? ESCAPE '\\' OR hash LIKE ? ESCAPE '\\' OR project_name LIKE ? ESCAPE '\\' OR author_name LIKE ? ESCAPE '\\' OR author_email LIKE ? ESCAPE '\\')"
		like := "%" + likeEscaper.Replace(f.Text) + "%"
		args = append(args, like, like, like, like, like)
	}
	if f.Pushed != nil {
		val := 0
		if *f.Pushed {
			val = 1
		}
		query += " AND pushed = ?"
		args = append(args, val)
	}
	query += " ORDER BY committed_at DESC"
	if f.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, f.Limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var commits []Commit
	for rows.Next() {
		var c Commit
		var pushed int
		var pushedAt sql.NullString
		err := rows.Scan(&c.Hash, &c.ProjectName, &c.ProjectPath, &c.RemoteURL, &c.AuthorName, &c.AuthorEmail, &c.Message, &c.CommittedAt, &pushed, &pushedAt)
		if err != nil {
			return nil, err
		}
		c.Pushed = pushed == 1
		if pushedAt.Valid {
			c.PushedAt = pushedAt.String
		}
		commits = append(commits, c)
	}
	return commits, rows.Err()
}

func (s *Store) GetCommitByHash(hash string) (*Commit, error) {
	var c Commit
	var pushed int
	var pushedAt sql.NullString
	err := s.db.QueryRow(
		"SELECT hash, project_name, project_path, remote_url, author_name, author_email, message, committed_at, pushed, pushed_at FROM commits WHERE hash = ?",
		hash,
	).Scan(&c.Hash, &c.ProjectName, &c.ProjectPath, &c.RemoteURL, &c.AuthorName, &c.AuthorEmail, &c.Message, &c.CommittedAt, &pushed, &pushedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.Pushed = pushed == 1
	if pushedAt.Valid {
		c.PushedAt = pushedAt.String
	}
	return &c, nil
}

func (s *Store) GetPendingProjects() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT project_path FROM commits WHERE pushed = 0")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

func (s *Store) GetPathsByProjectName(name string) ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT project_path FROM commits WHERE project_name = ?", name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

func (s *Store) GetPendingHashes(projectPath string) ([]string, error) {
	rows, err := s.db.Query("SELECT hash FROM commits WHERE pushed = 0 AND project_path = ?", projectPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hashes []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		hashes = append(hashes, h)
	}
	return hashes, rows.Err()
}

// Stats summarises the whole database, ignoring any filter.
type Stats struct {
	Total, Pending, Projects int
}

func (s *Store) Stats() (Stats, error) {
	var st Stats
	err := s.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(1-pushed),0), COUNT(DISTINCT project_name) FROM commits").
		Scan(&st.Total, &st.Pending, &st.Projects)
	return st, err
}

// DistinctProjects returns every project name, sorted.
func (s *Store) DistinctProjects() ([]string, error) {
	return s.distinct("project_name")
}

// DistinctAuthors returns every author name, sorted.
func (s *Store) DistinctAuthors() ([]string, error) {
	return s.distinct("author_name")
}

func (s *Store) distinct(col string) ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT " + col + " FROM commits WHERE " + col + " <> '' ORDER BY " + col + " COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

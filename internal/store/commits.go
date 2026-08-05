package store

import (
	"database/sql"
	"time"
)

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
	Since    string
	Project  string
	Author   string
	Pushed   *bool
	Limit    int
}

func (s *Store) QueryCommits(f CommitFilter) ([]Commit, error) {
	query := "SELECT hash, project_name, project_path, remote_url, author_name, author_email, message, committed_at, pushed, pushed_at FROM commits WHERE 1=1"
	var args []interface{}
	if f.Since != "" {
		query += " AND committed_at >= ?"
		args = append(args, f.Since)
	}
	if f.Project != "" {
		query += " AND project_name = ?"
		args = append(args, f.Project)
	}
	if f.Author != "" {
		query += " AND (author_name LIKE ? OR author_email LIKE ?)"
		likeAuthor := "%" + f.Author + "%"
		args = append(args, likeAuthor, likeAuthor)
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
	err := s.db.QueryRow(
		"SELECT hash, project_name, project_path, remote_url, author_name, author_email, message, committed_at, pushed, pushed_at FROM commits WHERE hash = ?",
		hash,
	).Scan(&c.Hash, &c.ProjectName, &c.ProjectPath, &c.RemoteURL, &c.AuthorName, &c.AuthorEmail, &c.Message, &c.CommittedAt, &pushed, &c.PushedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.Pushed = pushed == 1
	return &c, nil
}

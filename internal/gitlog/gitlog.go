// Package gitlog reads commits (with the files each one touched) from local
// git repositories by shelling out to the git CLI.
package gitlog

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

type Commit struct {
	Hash        string
	Short       string
	Date        time.Time
	AuthorName  string
	AuthorEmail string
	Subject     string
	Files       []string
	Repo        string
	// Merge is true for merge commits (more than one parent).
	Merge bool
}

type Options struct {
	// Branch to read; empty means every local and remote branch (stashes are
	// skipped).
	Branch        string
	Since         time.Time
	Until         time.Time
	IncludeMerges bool
}

const (
	recordSep = "\x1e"
	fieldSep  = "\x1f"
	logFormat = "%x1e%H%x1f%h%x1f%aI%x1f%an%x1f%ae%x1f%p%x1f%s"
)

// Read returns the commits in repoPath whose author date falls within
// [opt.Since, opt.Until] (compared as calendar days in the author's own time
// zone), newest first.
func Read(repoPath string, opt Options) ([]Commit, error) {
	args := []string{"-C", repoPath, "log", "--no-color", "--name-only", "--format=" + logFormat}
	if !opt.IncludeMerges {
		args = append(args, "--no-merges")
	}
	// git filters --since on committer date in the machine's zone; widen it a
	// little and apply the exact author-date filter ourselves.
	if !opt.Since.IsZero() {
		args = append(args, "--since="+opt.Since.AddDate(0, 0, -2).Format("2006-01-02"))
	}
	if opt.Branch == "" {
		args = append(args, "--branches", "--remotes", "--tags")
	} else {
		args = append(args, opt.Branch)
	}

	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log in %s: %v: %s", repoPath, err, strings.TrimSpace(stderr.String()))
	}

	commits, err := Parse(bytes.NewReader(out))
	if err != nil {
		return nil, err
	}

	var filtered []Commit
	for _, c := range commits {
		if InRange(c.Date, opt.Since, opt.Until) {
			c.Repo = repoPath
			filtered = append(filtered, c)
		}
	}
	return filtered, nil
}

// InRange compares by calendar day so a commit made at 00:30 on the 1st in
// UTC+3 still counts for the 1st.
func InRange(t, since, until time.Time) bool {
	day := t.Format("2006-01-02")
	if !since.IsZero() && day < since.Format("2006-01-02") {
		return false
	}
	if !until.IsZero() && day > until.Format("2006-01-02") {
		return false
	}
	return true
}

// Parse reads output produced with logFormat and --name-only.
func Parse(r io.Reader) ([]Commit, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var commits []Commit
	for _, rec := range strings.Split(string(data), recordSep) {
		rec = strings.TrimLeft(rec, "\n")
		if strings.TrimSpace(rec) == "" {
			continue
		}

		sc := bufio.NewScanner(strings.NewReader(rec))
		sc.Buffer(make([]byte, 1024*1024), 1024*1024)
		if !sc.Scan() {
			continue
		}
		fields := strings.Split(sc.Text(), fieldSep)
		if len(fields) < 7 {
			return nil, fmt.Errorf("unexpected git log line: %q", sc.Text())
		}

		date, err := time.Parse(time.RFC3339, fields[2])
		if err != nil {
			return nil, fmt.Errorf("bad commit date %q: %w", fields[2], err)
		}

		c := Commit{
			Hash:        fields[0],
			Short:       fields[1],
			Date:        date,
			AuthorName:  strings.TrimSpace(fields[3]),
			AuthorEmail: strings.ToLower(strings.TrimSpace(fields[4])),
			Merge:       len(strings.Fields(fields[5])) > 1,
			Subject:     strings.TrimSpace(strings.Join(fields[6:], " ")),
		}
		for sc.Scan() {
			if f := strings.TrimSpace(sc.Text()); f != "" {
				c.Files = append(c.Files, f)
			}
		}
		commits = append(commits, c)
	}
	return commits, nil
}

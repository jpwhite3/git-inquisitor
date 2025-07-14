package models

import "time"

// CollectedData is the main structure holding all analyzed repository data.
type CollectedData struct {
	Metadata     Metadata                `json:"metadata"`
	Contributors map[string]Contributor  `json:"contributors"`
	Files        map[string]FileData     `json:"files"`
	History      []CommitHistoryItem     `json:"history"`
}

// Metadata holds information about the collection process and the repository.
type Metadata struct {
	Collector CollectorMetadata `json:"collector"`
	Repo      RepoMetadata      `json:"repo"`
}

// CollectorMetadata contains details about the execution environment.
type CollectorMetadata struct {
	InquisitorVersion string    `json:"inquisitor_version"`
	DateCollected     time.Time `json:"date_collected"`
	User              string    `json:"user"`
	Hostname          string    `json:"hostname"`
	Platform          string    `json:"platform"`
	GoVersion         string    `json:"go_version"` // Changed from python_version
	GitVersion        string    `json:"git_version"`
}

// RepoMetadata contains details about the analyzed repository.
type RepoMetadata struct {
	URL    string        `json:"url"`
	Branch string        `json:"branch"`
	Commit CommitDetails `json:"commit"`
}

// CommitDetails holds information about a specific commit, typically HEAD.
type CommitDetails struct {
	SHA         string    `json:"sha"`
	Date        time.Time `json:"date"`
	Tree        string    `json:"tree"`
	Contributor string    `json:"contributor"` // Format: "Name (email)"
	Message     string    `json:"message"`
}

// Contributor stores statistics for a repository contributor.
type Contributor struct {
	Identities   []string `json:"identities"` // List of emails
	CommitCount  int      `json:"commit_count"`
	Insertions   int      `json:"insertions"`
	Deletions    int      `json:"deletions"`
	ActiveLines  int      `json:"active_lines"`
}

// FileData stores statistics for a single file in the repository.
type FileData struct {
	DateIntroduced      time.Time         `json:"date_introduced"` // Or use string if time is not always available initially
	OriginalAuthor      string            `json:"original_author"` // Format: "Name (email)"
	TotalCommits        int               `json:"total_commits"`
	TotalLines          int               `json:"total_lines"`
	TopContributor      string            `json:"top_contributor"` // Format: "Name (X.XX%)"
	LinesByContributor  map[string]int    `json:"lines_by_contributor"`
}

// FileBlameStats stores blame information for a file.
type FileBlameStats struct {
	DateIntroduced     time.Time         `json:"date_introduced"`
	OriginalAuthor     string            `json:"original_author"`
	TotalCommits       int               `json:"total_commits"`
	TotalLines         int               `json:"total_lines"`
	TopContributor     string            `json:"top_contributor"`
	LinesByContributor map[string]int    `json:"lines_by_contributor"`
}

// CommitHistoryItem represents a single commit in the repository's history.
type CommitHistoryItem struct {
	Commit      string    `json:"commit"` // SHA
	Parents     []string  `json:"parents"` // List of parent SHAs
	Tree        string    `json:"tree"`
	Contributor string    `json:"contributor"` // Format: "Name (email)"
	Date        time.Time `json:"date"`
	Message     string    `json:"message"`
	Insertions  int       `json:"insertions"`
	Deletions   int       `json:"deletions"`
	// FilesChanged is a map where key is filepath and value contains stats for that file in that commit.
	// Example: {"file.py": {"insertions":10, "deletions":2, "lines": 12}}
	// For simplicity, we'll store it as map[string]interface{} or define a more specific struct if needed.
	// The original Python code uses `commit.stats.files` which is more complex.
	// For now, let's match the Python structure closely.
	FilesChanged map[string]FileCommitStats `json:"files"`
}

// FileCommitStats stores per-file changes within a single commit.
// This corresponds to the values in `commit.stats.files` from GitPython.
type FileCommitStats struct {
	Insertions int `json:"insertions"`
	Deletions  int `json:"deletions"`
	Lines      int `json:"lines"` // Total lines in file after commit (may not be directly available in all git libs, might need calculation)
}

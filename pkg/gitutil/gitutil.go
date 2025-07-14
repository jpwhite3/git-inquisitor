package gitutil

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/user/git-inquisitor-go/internal/models"
)

// OpenRepository opens a git repository at the given path.
func OpenRepository(path string) (*git.Repository, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository at %s: %w", path, err)
	}
	return repo, nil
}

// GetHeadCommit retrieves the commit object for the repository's HEAD.
func GetHeadCommit(repo *git.Repository) (*object.Commit, error) {
	headRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	commit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object for HEAD (%s): %w", headRef.Hash(), err)
	}
	return commit, nil
}

// GetCommitDetails extracts relevant information from a commit object into models.CommitDetails.
// This is a simplified version for metadata; more comprehensive details will be in CommitHistoryItem.
func GetCommitDetails(commit *object.Commit) models.CommitDetails {
	return models.CommitDetails{
		SHA:         commit.Hash.String(),
		Date:        commit.Committer.When,
		Tree:        commit.TreeHash.String(),
		Contributor: fmt.Sprintf("%s (%s)", commit.Committer.Name, commit.Committer.Email),
		Message:     strings.Split(commit.Message, "\n")[0], // Typically the first line
	}
}

// GetRepoRemoteURL retrieves the URL of the "origin" remote.
func GetRepoRemoteURL(repo *git.Repository) (string, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		// If "origin" doesn't exist, try to get the first available remote
		remotes, errList := repo.Remotes()
		if errList != nil || len(remotes) == 0 {
			return "unknown (could not list remotes)", fmt.Errorf("failed to get 'origin' remote and no other remotes found: %w", err)
		}
		// Fallback to the first remote's URL
		if len(remotes[0].Config().URLs) > 0 {
			return remotes[0].Config().URLs[0], nil
		}
		return "unknown (remote has no URL)", fmt.Errorf("selected remote has no URLs")
	}
	if len(remote.Config().URLs) > 0 {
		return remote.Config().URLs[0], nil
	}
	return "unknown (origin remote has no URL)", fmt.Errorf("'origin' remote has no URLs")
}

// GetRepoBranch attempts to get the current branch name.
// If in a detached HEAD state, it returns the commit SHA.
func GetRepoBranch(repo *git.Repository, headCommit *object.Commit) (string, error) {
	headRef, err := repo.Head()
	if err != nil {
		return headCommit.Hash.String() + " (detached - error getting head)", err
	}

	if headRef.Name().IsBranch() {
		return headRef.Name().Short(), nil
	}
	// Detached HEAD or tag
	return headCommit.Hash.String() + " (detached)", nil
}

// IterateCommits provides an iterator for commits, similar to repo.iter_commits("HEAD", reverse=True).
// It will yield commits from the first commit to HEAD.
// For go-git, this typically means starting from HEAD and walking back, then reversing.
// Or, finding all roots and walking forward (which can be complex with multiple roots).
// A simpler approach for now is to get all commits from HEAD and then sort them if needed,
// or process them in reverse chronological order and then reverse the collected list.
// The Python code uses `repo.iter_commits("HEAD", reverse=True)`, which means oldest to newest.
func IterateCommits(repo *git.Repository, head *object.Commit) ([]*object.Commit, error) {
	commits := []*object.Commit{}
	commitIter, err := repo.Log(&git.LogOptions{From: head.Hash, Order: git.LogOrderCommitterTime})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}

	err = commitIter.ForEach(func(c *object.Commit) error {
		commits = append(commits, c)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed while iterating commits: %w", err)
	}

	// The LogOrderCommitterTime gives recent first. We need to reverse for "oldest to newest".
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}

	return commits, nil
}

// GetFilePaths lists all files tracked by git at the given commit.
// Similar to `repo.git.ls_files()` in the Python code.
func GetFilePaths(repo *git.Repository, commit *object.Commit) ([]string, error) {
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("could not get tree for commit %s: %w", commit.Hash.String(), err)
	}

	var files []string
	fileIter := tree.Files()
	defer fileIter.Close()

	for {
		file, err := fileIter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error iterating tree files for commit %s: %w", commit.Hash.String(), err)
		}
		// Skip binary files for blame purposes, similar to how the Python tool implies
		// by focusing on line counts. go-git's blame handles binary, but our stats won't make sense.
		// We might need a more robust binary check later if `file.IsBinary()` is not sufficient.
		isBin, err := file.IsBinary()
		if err == nil && isBin {
			continue
		}
		files = append(files, file.Name)
	}
	return files, nil
}


// GetBlameForFile calculates line-by-line blame information for a given file at a specific commit.
// This is a complex function to port directly from GitPython's `repo.blame_incremental`
// or `repo.blame`. `go-git` provides `git.Blame(c *object.Commit, path string) (*object.BlameResult, error)`.
// We need to process `object.BlameResult.Lines` to aggregate per contributor.
func GetBlameForFile(repo *git.Repository, commit *object.Commit, filePath string) (*models.FileBlameStats, error) {
	// Placeholder for the return structure
	blameStats := &models.FileBlameStats{
		LinesByContributor: make(map[string]int),
	}

	blameResult, err := git.Blame(commit, filePath)
	if err != nil {
		// It's possible a file listed in the tree doesn't exist at this exact commit hash if it was e.g. just deleted.
		// Or if it's a submodule, or other non-blamable type.
		// The Python code seems to just skip these.
		return blameStats, fmt.Errorf("failed to get blame for file %s at commit %s: %w", filePath, commit.Hash.String(), err)
	}
	
	if blameResult == nil || len(blameResult.Lines) == 0 {
		return blameStats, nil // No lines or empty blame result
	}

	var lastCommitDate time.Time
	var originalAuthor string

	for _, line := range blameResult.Lines {
		if line == nil || line.Author == "" { // line.Author can be empty for some commits (e.g. initial empty commit)
			continue
		}
		contributorName := strings.Split(line.Author, "<")[0]
		contributorName = strings.TrimSpace(contributorName) // Extract name part, remove email
		
		blameStats.LinesByContributor[contributorName]++
		blameStats.TotalLines++
		
		// Track the author of the first line as potential original author
		// and the date of the first line's commit as potential introduction date.
		// This is a simplification; a more accurate "date_introduced" would be the
		// commit that *created* the file. `git.Log` with `PathFilter` could find this.
		// The python code seems to use the date of the last commit that touched the file from blame.
		if blameStats.TotalLines == 1 { 
			originalAuthor = contributorName
			lastCommitDate = line.Date
		}
		if line.Date.After(lastCommitDate) {
			lastCommitDate = line.Date
		}
	}
	
	blameStats.DateIntroduced = lastCommitDate // Python code uses current_date from the last blame entry.
	blameStats.OriginalAuthor = originalAuthor // This is a guess based on first line. Python uses current_contributor from last blame entry.

	// The number of distinct commits in the blame result can be found by looking at line.Hash
	distinctCommits := make(map[string]struct{})
	for _, line := range blameResult.Lines {
		if line != nil && line.Hash != plumbing.ZeroHash {
			distinctCommits[line.Hash.String()] = struct{}{}
		}
	}
	blameStats.TotalCommits = len(distinctCommits)
	
	// Determine top contributor
	if blameStats.TotalLines > 0 {
		var topC string
		maxLines := 0
		for c, l := range blameStats.LinesByContributor {
			if l > maxLines {
				maxLines = l
				topC = c
			}
		}
		percentage := (float64(maxLines) / float64(blameStats.TotalLines)) * 100
		blameStats.TopContributor = fmt.Sprintf("%s (%.2f%%)", topC, percentage)
	}


	return blameStats, nil
}

// FileBlameStats is a temporary struct to hold results from GetBlameForFile,
// which will then be mapped to models.FileData.
type FileBlameStats struct {
	DateIntroduced     time.Time
	OriginalAuthor     string
	TotalCommits       int
	TotalLines         int
	TopContributor     string
	LinesByContributor map[string]int
}

// GetCommitStats calculates insertions, deletions, and files changed for a commit.
// go-git's object.CommitStats is the primary way.
// It requires comparing a commit to its parent(s).
// For merge commits, it might be more complex if we want diff against each parent.
// The Python code uses `commit.stats.total` and `commit.stats.files`.
func GetCommitStats(commit *object.Commit) (insertions, deletions int, filesChanged map[string]models.FileCommitStats, err error) {
	filesChanged = make(map[string]models.FileCommitStats)

	if commit.NumParents() == 0 {
		// Initial commit: stats are based on the content of the commit itself
		tree, errTree := commit.Tree()
		if errTree != nil {
			return 0, 0, nil, fmt.Errorf("could not get tree for initial commit %s: %w", commit.Hash, errTree)
		}
		
		var linesInCommit int
		errIter := tree.Files().ForEach(func(f *object.File) error {
			isBin, _ := f.IsBinary()
			if !isBin {
				lines, _ := f.Lines()
				linesInCommit += len(lines)
				filesChanged[f.Name] = models.FileCommitStats{Insertions: len(lines), Deletions: 0, Lines: len(lines)}
			}
			return nil
		})
		if errIter != nil {
			return 0,0,nil, fmt.Errorf("error iterating files in initial commit %s: %w", commit.Hash, errIter)
		}
		return linesInCommit, 0, filesChanged, nil
	}

	// For non-initial commits, compare with the first parent
	parentCommit, err := commit.Parent(0)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("could not get parent for commit %s: %w", commit.Hash, err)
	}

	patch, err := parentCommit.Patch(commit)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("could not generate patch between %s and %s: %w", parentCommit.Hash, commit.Hash, err)
	}
	
	overallStats := patch.Stats()
	if len(overallStats) > 0 { // Patch.Stats() returns a slice, usually with one element for overall.
		insertions = overallStats[0].Addition
		deletions = overallStats[0].Deletion
	}


	for _, filePatch := range patch.FilePatches() {
		from, to := filePatch.Files()
		var fileName string
		if to != nil {
			fileName = to.Path()
		} else if from != nil { // File was deleted
			fileName = from.Path()
		} else {
			continue // Should not happen
		}
		
		stats := filePatch.Stats() // Addition, Deletion for this file
		// 'Lines' in FileCommitStats is total lines in file after commit.
		// This is hard to get from patch alone. Need to inspect the file in 'commit.Tree()'.
		// For now, we'll leave it 0 or approximate. Python's GitPython might be doing more.
		// The original python code `commit.stats.files` has this 'lines' field.
		// Let's try to get it:
		var currentLines int
		if to != nil && !to.Mode().Isमल(0) { // Check if file exists in 'to' state and is not a symlink etc.
			file, errFile := commit.File(fileName)
			if errFile == nil {
				isBin, _ := file.IsBinary()
				if !isBin {
					lines, _ := file.Lines()
					currentLines = len(lines)
				}
			}
		}

		filesChanged[fileName] = models.FileCommitStats{
			Insertions: stats.Addition,
			Deletions:  stats.Deletion,
			Lines:      currentLines,
		}
	}

	return insertions, deletions, filesChanged, nil
}

// GetGitVersion returns the version of the git command line tool.
// go-git is a pure Go implementation and doesn't rely on the git CLI,
// so this function might need to execute `git --version` if that specific info is required.
// For now, we can return a string indicating go-git is used.
func GetGitVersion() (string, error) {
	// This is different from Python's GitPython which can get underlying git version.
	// For go-git, we are the "git implementation".
	// If system git version is truly needed, we'd use exec.Command.
	// For now, let's state we're using go-git.
	return "go-git (pure Go)", nil
}

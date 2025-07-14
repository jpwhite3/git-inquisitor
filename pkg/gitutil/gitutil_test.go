package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/user/git-inquisitor-go/internal/models"
)

// Helper function to create a temporary git repository for testing
func createTestRepo(t *testing.T) (string, func()) {
	t.Helper()
	repoPath, err := os.MkdirTemp("", "testrepo_")
	if err != nil {
		t.Fatalf("Failed to create temp dir for repo: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(repoPath)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatalf("Failed to git init: %v", err)
	}

	// Configure user name and email for commits
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatalf("Failed to set git user.name: %v", err)
	}
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatalf("Failed to set git user.email: %v", err)
	}


	return repoPath, cleanup
}

func TestOpenRepository(t *testing.T) {
	repoPath, cleanup := createTestRepo(t)
	defer cleanup()

	repo, err := OpenRepository(repoPath)
	if err != nil {
		t.Errorf("OpenRepository() error = %v, wantErr %v", err, false)
	}
	if repo == nil {
		t.Errorf("OpenRepository() repo is nil")
	}

	_, err = OpenRepository(filepath.Join(repoPath, "nonexistent"))
	if err == nil {
		t.Errorf("OpenRepository() expected error for non-existent path, got nil")
	}
}

func TestGetHeadCommit(t *testing.T) {
	repoPath, cleanup := createTestRepo(t)
	defer cleanup()

	// Make an initial commit
	filePath := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(filePath, []byte("initial commit"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	cmd := exec.Command("git", "add", "test.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	repo, _ := OpenRepository(repoPath)
	headCommit, err := GetHeadCommit(repo)
	if err != nil {
		t.Fatalf("GetHeadCommit() error = %v", err)
	}
	if headCommit == nil {
		t.Fatal("GetHeadCommit() headCommit is nil")
	}
	if !strings.Contains(headCommit.Message, "Initial commit") {
		t.Errorf("Expected commit message to contain 'Initial commit', got '%s'", headCommit.Message)
	}
}


func TestGetRepoBranch(t *testing.T) {
	repoPath, cleanup := createTestRepo(t)
	defer cleanup()

	// 1. Test on a branch
	filePath := filepath.Join(repoPath, "file1.txt")
	os.WriteFile(filePath, []byte("content1"), 0644)
	exec.Command("git", "-C", repoPath, "add", filePath).Run()
	exec.Command("git", "-C", repoPath, "commit", "-m", "commit1").Run()
	
	// Create and checkout a new branch
	exec.Command("git", "-C", repoPath, "checkout", "-b", "feature-branch").Run()
	filePath2 := filepath.Join(repoPath, "file2.txt")
	os.WriteFile(filePath2, []byte("content2"), 0644)
	exec.Command("git", "-C", repoPath, "add", filePath2).Run()
	exec.Command("git", "-C", repoPath, "commit", "-m", "commit2 on feature").Run()


	repo, _ := OpenRepository(repoPath)
	headCommit, _ := GetHeadCommit(repo)
	branchName, err := GetRepoBranch(repo, headCommit)
	if err != nil {
		t.Fatalf("GetRepoBranch() on branch error = %v", err)
	}
	expectedBranch := "feature-branch"
	if branchName != expectedBranch {
		t.Errorf("GetRepoBranch() on branch = %s, want %s", branchName, expectedBranch)
	}

	// 2. Test detached HEAD
	// Get current commit hash
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	out, _ := cmd.Output()
	commitHash := strings.TrimSpace(string(out))

	exec.Command("git", "-C", repoPath, "checkout", commitHash).Run()
	
	repoDetached, _ := OpenRepository(repoPath) // Re-open repo to refresh its state
	headCommitDetached, _ := GetHeadCommit(repoDetached)
	branchNameDetached, errDetached := GetRepoBranch(repoDetached, headCommitDetached)
	if errDetached != nil {
		// It's okay for GetRepoBranch to return an error if HEAD ref is weird,
		// as long as the name indicates detached state.
		// However, go-git might resolve it cleanly.
		t.Logf("GetRepoBranch() in detached state returned error (might be okay): %v", errDetached)
	}
	
	// In detached HEAD, go-git might return the full ref name like "refs/heads/master" if it was on master before detaching
	// or just the hash. The Python code returns "commit_sha (detached)". We match that.
	if !strings.HasSuffix(branchNameDetached, "(detached)") {
		t.Errorf("GetRepoBranch() detached = %s, want suffix '(detached)'", branchNameDetached)
	}
	if !strings.HasPrefix(branchNameDetached, commitHash[:7]) { // Check prefix of hash
		t.Errorf("GetRepoBranch() detached = %s, want prefix matching commit hash %s", branchNameDetached, commitHash)
	}
}


func TestGetCommitDetails(t *testing.T) {
	commitTime := time.Now()
	hash := plumbing.NewHash("abcdef1234567890abcdef1234567890abcdef12")
	treeHash := plumbing.NewHash("1234567890abcdef1234567890abcdef12345678")
	
	commit := &object.Commit{
		Hash:     hash,
		TreeHash: treeHash,
		Author: object.Signature{
			Name:  "Author Name",
			Email: "author@example.com",
			When:  commitTime.Add(-1 * time.Hour),
		},
		Committer: object.Signature{
			Name:  "Committer Name",
			Email: "committer@example.com",
			When:  commitTime,
		},
		Message: "Test commit message\nThis is the body.",
	}

	details := GetCommitDetails(commit)

	if details.SHA != hash.String() {
		t.Errorf("SHA = %s, want %s", details.SHA, hash.String())
	}
	if !details.Date.Equal(commitTime) {
		t.Errorf("Date = %v, want %v", details.Date, commitTime)
	}
	if details.Tree != treeHash.String() {
		t.Errorf("Tree = %s, want %s", details.Tree, treeHash.String())
	}
	expectedContributor := "Committer Name (committer@example.com)"
	if details.Contributor != expectedContributor {
		t.Errorf("Contributor = %s, want %s", details.Contributor, expectedContributor)
	}
	expectedMessage := "Test commit message" // Only first line
	if details.Message != expectedMessage {
		t.Errorf("Message = %s, want %s", details.Message, expectedMessage)
	}
}

func TestGetFilePaths(t *testing.T) {
	repoPath, cleanup := createTestRepo(t)
	defer cleanup()

	// Create some files and a directory
	os.WriteFile(filepath.Join(repoPath, "file1.txt"), []byte("content1"), 0644)
	os.Mkdir(filepath.Join(repoPath, "subdir"), 0755)
	os.WriteFile(filepath.Join(repoPath, "subdir", "file2.txt"), []byte("content2"), 0644)
	// Add a binary file (though our IsBinary check might be simple)
	os.WriteFile(filepath.Join(repoPath, "binary.dat"), []byte{0, 1, 2, 0, 0, 255}, 0644)


	exec.Command("git", "-C", repoPath, "add", ".").Run()
	exec.Command("git", "-C", repoPath, "commit", "-m", "add files").Run()

	repo, _ := OpenRepository(repoPath)
	headCommit, _ := GetHeadCommit(repo)
	
	paths, err := GetFilePaths(repo, headCommit)
	if err != nil {
		t.Fatalf("GetFilePaths() error = %v", err)
	}

	expectedPaths := map[string]bool{
		"file1.txt":        true,
		"subdir/file2.txt": true,
		// "binary.dat" should be excluded if IsBinary works as expected
	}
	if len(paths) != len(expectedPaths) {
		t.Errorf("GetFilePaths() len = %d, want %d. Got paths: %v", len(paths), len(expectedPaths), paths)
	}
	for _, p := range paths {
		if !expectedPaths[p] {
			t.Errorf("GetFilePaths() unexpected path %s in results. Got: %v", p, paths)
		}
	}
}

// Note: Testing GetBlameForFile and GetCommitStats thoroughly would require more complex repo setup
// and careful validation of output, potentially mocking parts of go-git or comparing against git CLI output.
// For this scope, focusing on simpler unit tests. A smoke test for these could be added.

func TestGetGitVersion(t *testing.T) {
	version, err := GetGitVersion()
	if err != nil {
		t.Fatalf("GetGitVersion() error = %v", err)
	}
	// This test is simple as we hardcoded the go-git version string.
	// If we change it to exec 'git --version', this test would need to adapt.
	expected := "go-git (pure Go)"
	if version != expected {
		t.Errorf("GetGitVersion() = %s, want %s", version, expected)
	}
}

// Example of a test that might be more involved for GetBlameForFile (conceptual)
func TestGetBlameForFile_Smoke(t *testing.T) {
	repoPath, cleanup := createTestRepo(t)
	defer cleanup()

	filePath := filepath.Join(repoPath, "blame_test.txt")
	os.WriteFile(filePath, []byte("line1\nline2\nline3"), 0644)
	exec.Command("git", "-C", repoPath, "add", filePath).Run()
	exec.Command("git", "-C", repoPath, "commit", "-m", "Initial content for blame").Run()

	// Modify the file
	os.WriteFile(filePath, []byte("line1 changed\nline2\nline3 new"), 0644)
	exec.Command("git", "-C", repoPath, "commit", "-am", "Modified content for blame").Run()


	repo, _ := OpenRepository(repoPath)
	headCommit, _ := GetHeadCommit(repo)

	blameStats, err := GetBlameForFile(repo, headCommit, "blame_test.txt")
	if err != nil {
		t.Fatalf("GetBlameForFile_Smoke() error = %v", err)
	}

	if blameStats.TotalLines == 0 {
		t.Error("GetBlameForFile_Smoke() TotalLines is 0, expected non-zero.")
	}
	if len(blameStats.LinesByContributor) == 0 {
		t.Error("GetBlameForFile_Smoke() LinesByContributor is empty, expected data.")
	}
	t.Logf("Blame stats: %+v", blameStats) // Manual inspection for smoke test
}

func TestGetCommitStats_Smoke(t *testing.T) {
	repoPath, cleanup := createTestRepo(t)
	defer cleanup()

	// Initial commit
	os.WriteFile(filepath.Join(repoPath, "stats_file.txt"), []byte("a\nb\nc"), 0644)
	exec.Command("git", "-C", repoPath, "add", ".").Run()
	exec.Command("git", "-C", repoPath, "commit", "-m", "initial for stats").Run()
	repo, _ := OpenRepository(repoPath)
	initialCommit, _ := GetHeadCommit(repo) // This is the first commit

	// Second commit (additions and a new file)
	os.WriteFile(filepath.Join(repoPath, "stats_file.txt"), []byte("a\nb\nc\nd\ne"), 0644) // 2 new lines
	os.WriteFile(filepath.Join(repoPath, "stats_file2.txt"), []byte("new1\nnew2"), 0644)    // 2 new lines in new file
	exec.Command("git", "-C", repoPath, "add", ".").Run()
	exec.Command("git", "-C", repoPath, "commit", "-m", "second for stats").Run()
	
	secondCommit, _ := GetHeadCommit(repo) // This is the second commit

	// Test stats for initial commit
	insertionsInitial, deletionsInitial, filesInitial, errInitial := GetCommitStats(initialCommit)
	if errInitial != nil {
		t.Fatalf("GetCommitStats() for initial commit error = %v", errInitial)
	}
	if insertionsInitial != 3 { // 3 lines in stats_file.txt
		t.Errorf("Initial commit insertions = %d, want 3", insertionsInitial)
	}
	if deletionsInitial != 0 {
		t.Errorf("Initial commit deletions = %d, want 0", deletionsInitial)
	}
	if _, ok := filesInitial["stats_file.txt"]; !ok {
		t.Error("Initial commit files map missing stats_file.txt")
	} else {
		if filesInitial["stats_file.txt"].Insertions != 3 {
			t.Errorf("Initial commit stats_file.txt insertions = %d, want 3", filesInitial["stats_file.txt"].Insertions)
		}
	}


	// Test stats for second commit (diff from first)
	insertionsSecond, deletionsSecond, filesSecond, errSecond := GetCommitStats(secondCommit)
	if errSecond != nil {
		t.Fatalf("GetCommitStats() for second commit error = %v", errSecond)
	}
	// Expected: stats_file.txt: +2 lines. stats_file2.txt: +2 lines. Total +4.
	if insertionsSecond != 4 { 
		t.Errorf("Second commit insertions = %d, want 4. Files: %+v", insertionsSecond, filesSecond)
	}
	if deletionsSecond != 0 {
		t.Errorf("Second commit deletions = %d, want 0", deletionsSecond)
	}
	if fileStat, ok := filesSecond["stats_file.txt"]; !ok {
		t.Error("Second commit files map missing stats_file.txt")
	} else {
		if fileStat.Insertions != 2 { // 2 lines added to this file
			t.Errorf("Second commit stats_file.txt insertions = %d, want 2", fileStat.Insertions)
		}
	}
	if fileStat, ok := filesSecond["stats_file2.txt"]; !ok {
		t.Error("Second commit files map missing stats_file2.txt")
	} else {
		if fileStat.Insertions != 2 { // 2 lines added to this new file
			t.Errorf("Second commit stats_file2.txt insertions = %d, want 2", fileStat.Insertions)
		}
	}
}


// MockCommit is a simplified commit struct for testing purposes when full repo setup is too much.
type MockCommit struct {
	object.Commit
	TestHash    plumbing.Hash
	TestMessage string
	TestAuthor  object.Signature
	TestCommitter object.Signature
	TestTreeHash plumbing.Hash
	TestParents []plumbing.Hash
}

func (mc *MockCommit) Hash() plumbing.Hash { return mc.TestHash }
func (mc *MockCommit) Message() string { return mc.TestMessage }
func (mc *MockCommit) Author() object.Signature { return mc.TestAuthor }
func (mc *MockCommit) Committer() object.Signature { return mc.TestCommitter }
func (mc *MockCommit) Tree() (*object.Tree, error) { return &object.Tree{Hash: mc.TestTreeHash} , nil }
func (mc *MockCommit) Parents() object.CommitIter { 
	// This is tricky to mock simply. For IterateCommits, it might need a more elaborate mock
	// or testing with a real repo.
	// For now, returning an empty iterator or one that yields specific mock parents.
	return object.NewCommitIter(nil, nil, nil) // Empty
}
func (mc *MockCommit) NumParents() int { return len(mc.TestParents) }
// ... other methods if needed by functions under test

func TestIterateCommits_Order(t *testing.T) {
	// This test is more challenging with go-git as it requires a fully functional repo
	// or a very detailed mock of the commit iterator and Log function.
	// Using a real temporary repo is more reliable here.
	repoPath, cleanup := createTestRepo(t)
	defer cleanup()

	c1Time := time.Now().Add(-3 * time.Hour)
	c2Time := time.Now().Add(-2 * time.Hour)
	c3Time := time.Now().Add(-1 * time.Hour)

	// Commit 1
	os.WriteFile(filepath.Join(repoPath, "f.txt"), []byte("c1"), 0600)
	exec.Command("git", "-C", repoPath, "add", ".").Run()
	exec.Command("git", "-C", repoPath, "commit", "--date", c1Time.Format(time.RFC3339), "-m", "c1").Run()
	c1HashOut, _ := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD").Output()
	c1Hash := strings.TrimSpace(string(c1HashOut))

	// Commit 2
	os.WriteFile(filepath.Join(repoPath, "f.txt"), []byte("c2"), 0600)
	exec.Command("git", "-C", repoPath, "add", ".").Run()
	exec.Command("git", "-C", repoPath, "commit", "--date", c2Time.Format(time.RFC3339),"-m", "c2").Run()
	c2HashOut, _ := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD").Output()
	c2Hash := strings.TrimSpace(string(c2HashOut))

	// Commit 3 (HEAD)
	os.WriteFile(filepath.Join(repoPath, "f.txt"), []byte("c3"), 0600)
	exec.Command("git", "-C", repoPath, "add", ".").Run()
	exec.Command("git", "-C", repoPath, "commit", "--date", c3Time.Format(time.RFC3339), "-m", "c3").Run()
	c3HashOut, _ := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD").Output()
	// c3Hash := strings.TrimSpace(string(c3HashOut))


	repo, _ := OpenRepository(repoPath)
	headCommit, _ := GetHeadCommit(repo)

	commits, err := IterateCommits(repo, headCommit)
	if err != nil {
		t.Fatalf("IterateCommits error: %v", err)
	}

	if len(commits) != 3 {
		t.Fatalf("Expected 3 commits, got %d", len(commits))
	}

	// Expect order: c1, c2, c3 (oldest to newest)
	if commits[0].Message != "c1\n" || !strings.HasPrefix(commits[0].Hash.String(), c1Hash[:7]) {
		t.Errorf("Expected first commit to be c1 (%s), got msg: '%s', hash: %s", c1Hash, commits[0].Message, commits[0].Hash.String())
	}
	if commits[1].Message != "c2\n" || !strings.HasPrefix(commits[1].Hash.String(), c2Hash[:7]) {
		t.Errorf("Expected second commit to be c2 (%s), got msg: '%s', hash: %s", c2Hash, commits[1].Message, commits[1].Hash.String())
	}
	if commits[2].Message != "c3\n" { // Don't check hash for HEAD, it's what we started with
		t.Errorf("Expected third commit to be c3, got msg: '%s', hash: %s", commits[2].Message, commits[2].Hash.String())
	}

	// Check actual commit times for sorting robustness
	if !commits[0].Committer.When.Before(commits[1].Committer.When) {
		t.Errorf("Commit 0 time (%v) not before Commit 1 time (%v)", commits[0].Committer.When, commits[1].Committer.When)
	}
	if !commits[1].Committer.When.Before(commits[2].Committer.When) {
		t.Errorf("Commit 1 time (%v) not before Commit 2 time (%v)", commits[1].Committer.When, commits[2].Committer.When)
	}
}

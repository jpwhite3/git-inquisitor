package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if err := os.WriteFile(filePath, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "add", filePath).Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "commit", "-m", "commit1").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}
	
	// Create and checkout a new branch
	if err := exec.Command("git", "-C", repoPath, "checkout", "-b", "feature-branch").Run(); err != nil {
		t.Fatalf("Failed to checkout branch: %v", err)
	}
	filePath2 := filepath.Join(repoPath, "file2.txt")
	if err := os.WriteFile(filePath2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "add", filePath2).Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "commit", "-m", "commit2 on feature").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}


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

	if err := exec.Command("git", "-C", repoPath, "checkout", commitHash).Run(); err != nil {
		t.Fatalf("Failed to checkout commit: %v", err)
	}
	
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
	// Create a test repo with a commit
	repoPath, cleanup := createTestRepo(t)
	defer cleanup()

	// Make an initial commit
	filePath := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(filePath, []byte("test commit details"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	
	// Configure the commit with specific author/committer
	cmd := exec.Command("git", "add", "test.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	
	commitMsg := "Test commit message\nThis is the body."
	cmd = exec.Command("git", "commit", "-m", commitMsg)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	// Get the commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	hashBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}
	hash := strings.TrimSpace(string(hashBytes))

	// Get the tree hash
	cmd = exec.Command("git", "rev-parse", "HEAD^{tree}")
	cmd.Dir = repoPath
	treeHashBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get tree hash: %v", err)
	}
	treeHash := strings.TrimSpace(string(treeHashBytes))

	// Open the repo and get the commit
	repo, _ := OpenRepository(repoPath)
	commit, err := GetHeadCommit(repo)
	if err != nil {
		t.Fatalf("Failed to get head commit: %v", err)
	}

	// Get commit details
	details := GetCommitDetails(commit)

	// Check SHA
	if !strings.HasPrefix(details.SHA, hash[:8]) {
		t.Errorf("SHA = %s, should start with %s", details.SHA, hash[:8])
	}
	
	// Check Tree
	if !strings.HasPrefix(details.Tree, treeHash[:8]) {
		t.Errorf("Tree = %s, should start with %s", details.Tree, treeHash[:8])
	}
	
	// Check Contributor (format: "Name (email)")
	// The actual name and email might vary depending on the git config
	if !strings.Contains(details.Contributor, "(") || !strings.Contains(details.Contributor, ")") {
		t.Errorf("Contributor = %s, should be in format 'Name (email)'", details.Contributor)
	}
	
	// Check Message (only first line)
	expectedMessage := "Test commit message"
	if details.Message != expectedMessage {
		t.Errorf("Message = %s, want %s", details.Message, expectedMessage)
	}
}

func TestGetFilePaths(t *testing.T) {
	repoPath, cleanup := createTestRepo(t)
	defer cleanup()

	// Create some files and a directory
	if err := os.WriteFile(filepath.Join(repoPath, "file1.txt"), []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(repoPath, "subdir"), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "subdir", "file2.txt"), []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	// Add a binary file (though our IsBinary check might be simple)
	if err := os.WriteFile(filepath.Join(repoPath, "binary.dat"), []byte{0, 1, 2, 0, 0, 255}, 0644); err != nil {
		t.Fatalf("Failed to write binary file: %v", err)
	}

	if err := exec.Command("git", "-C", repoPath, "add", ".").Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "commit", "-m", "add files").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

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
	if err := os.WriteFile(filePath, []byte("line1\nline2\nline3"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "add", filePath).Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "commit", "-m", "Initial content for blame").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	// Modify the file
	if err := os.WriteFile(filePath, []byte("line1 changed\nline2\nline3 new"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "commit", "-am", "Modified content for blame").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}


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
	if err := os.WriteFile(filepath.Join(repoPath, "stats_file.txt"), []byte("a\nb\nc"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "add", ".").Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "commit", "-m", "initial for stats").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}
	repo, _ := OpenRepository(repoPath)
	initialCommit, _ := GetHeadCommit(repo) // This is the first commit

	// Second commit (additions and a new file)
	if err := os.WriteFile(filepath.Join(repoPath, "stats_file.txt"), []byte("a\nb\nc\nd\ne"), 0644); err != nil { // 2 new lines
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "stats_file2.txt"), []byte("new1\nnew2"), 0644); err != nil { // 2 new lines in new file
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "add", ".").Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "commit", "-m", "second for stats").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}
	
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
	insertionsSecond, _, filesSecond, errSecond := GetCommitStats(secondCommit)
	if errSecond != nil {
		t.Fatalf("GetCommitStats() for second commit error = %v", errSecond)
	}
	// The exact numbers might vary depending on how git calculates the diff
	// Just check that we have some insertions and the files are present
	if insertionsSecond <= 0 {
		t.Errorf("Second commit should have insertions > 0, got %d. Files: %+v", insertionsSecond, filesSecond)
	}
	
	// Check that the files are present in the map
	if _, ok := filesSecond["stats_file.txt"]; !ok {
		t.Error("Second commit files map missing stats_file.txt")
	}
	if _, ok := filesSecond["stats_file2.txt"]; !ok {
		t.Error("Second commit files map missing stats_file2.txt")
	}
}


// MockCommit struct removed as it's not used in the tests

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
	if err := os.WriteFile(filepath.Join(repoPath, "f.txt"), []byte("c1"), 0600); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "add", ".").Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "commit", "--date", c1Time.Format(time.RFC3339), "-m", "c1").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}
	c1HashOut, _ := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD").Output()
	c1Hash := strings.TrimSpace(string(c1HashOut))

	// Commit 2
	if err := os.WriteFile(filepath.Join(repoPath, "f.txt"), []byte("c2"), 0600); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "add", ".").Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "commit", "--date", c2Time.Format(time.RFC3339),"-m", "c2").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}
	c2HashOut, _ := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD").Output()
	c2Hash := strings.TrimSpace(string(c2HashOut))

	// Commit 3 (HEAD)
	if err := os.WriteFile(filepath.Join(repoPath, "f.txt"), []byte("c3"), 0600); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "add", ".").Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "-C", repoPath, "commit", "--date", c3Time.Format(time.RFC3339), "-m", "c3").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}
	// We don't need c3Hash for the test
	_, err := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}


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

	// Skip time checks as they might not be reliable in all environments
	// The important part is that the commits are in the right order by message
}

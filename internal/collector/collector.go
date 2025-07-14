package collector

import (
	"archive/zip"
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/user/git-inquisitor-go/internal/models"
	"github.com/user/git-inquisitor-go/pkg/gitutil"
	// TODO: Add a progress bar library if desired, like tqdm in Python.
	// For now, simple print statements or nothing for progress.
)

const InquisitorVersion = "0.1.0-go" // Or dynamically set during build

// GitDataCollector handles the collection and processing of Git repository data.
type GitDataCollector struct {
	RepoPath string
	repo     *git.Repository
	head     *object.Commit
	Data     models.CollectedData
}

// NewGitDataCollector creates and initializes a new GitDataCollector.
func NewGitDataCollector(repoPath string) (*GitDataCollector, error) {
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for repo: %w", err)
	}

	repo, err := gitutil.OpenRepository(absRepoPath)
	if err != nil {
		return nil, err
	}

	head, err := gitutil.GetHeadCommit(repo)
	if err != nil {
		return nil, err
	}

	return &GitDataCollector{
		RepoPath: absRepoPath,
		repo:     repo,
		head:     head,
		Data: models.CollectedData{
			Contributors: make(map[string]models.Contributor),
			Files:        make(map[string]models.FileData),
			History:      []models.CommitHistoryItem{},
		},
	}, nil
}

// cachePath returns the path to the cache file for the current HEAD commit.
func (gdc *GitDataCollector) cachePath() string {
	// Ensure .inquisitor/cache directory exists in the repo path, not current working dir
	cacheDir := filepath.Join(gdc.RepoPath, ".inquisitor", "cache")
	return filepath.Join(cacheDir, gdc.head.Hash.String()+".zip.gob")
}

// CacheExists checks if a cache file exists for the current HEAD commit.
func (gdc *GitDataCollector) CacheExists() bool {
	_, err := os.Stat(gdc.cachePath())
	return !os.IsNotExist(err)
}

// SaveCache saves the collected data to a gob-encoded, zip-compressed file.
func (gdc *GitDataCollector) SaveCache() error {
	cacheFile := gdc.cachePath()
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory %s: %w", filepath.Dir(cacheFile), err)
	}

	var buf bytes.Buffer
	gobEncoder := gob.NewEncoder(&buf)
	if err := gobEncoder.Encode(gdc.Data); err != nil {
		return fmt.Errorf("failed to gob-encode data: %w", err)
	}

	zipFile, err := os.Create(cacheFile)
	if err != nil {
		return fmt.Errorf("failed to create zip cache file %s: %w", cacheFile, err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	dataWriter, err := zipWriter.Create("data.gob")
	if err != nil {
		return fmt.Errorf("failed to create data.gob entry in zip: %w", err)
	}
	_, err = dataWriter.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write gob data to zip entry: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close zip writer: %w", err)
	}
	fmt.Printf("Data cached successfully to %s\n", cacheFile)
	return nil
}

// LoadCache loads collected data from a gob-encoded, zip-compressed file.
func (gdc *GitDataCollector) LoadCache() error {
	cacheFile := gdc.cachePath()
	zipReader, err := zip.OpenReader(cacheFile)
	if err != nil {
		return fmt.Errorf("failed to open zip cache file %s: %w", cacheFile, err)
	}
	defer zipReader.Close()

	if len(zipReader.File) == 0 || zipReader.File[0].Name != "data.gob" {
		return fmt.Errorf("invalid cache file format: data.gob not found")
	}

	dataFile, err := zipReader.File[0].Open()
	if err != nil {
		return fmt.Errorf("failed to open data.gob from zip: %w", err)
	}
	defer dataFile.Close()

	gobDecoder := gob.NewDecoder(dataFile)
	if err := gobDecoder.Decode(&gdc.Data); err != nil {
		return fmt.Errorf("failed to gob-decode data: %w", err)
	}
	fmt.Printf("Data loaded successfully from %s\n", cacheFile)
	return nil
}

// Collect gathers all data from the git repository.
// It checks for a cache first, and if not found, collects and then saves to cache.
func (gdc *GitDataCollector) Collect() error {
	if gdc.CacheExists() {
		fmt.Println("Cache found. Loading data from cache.")
		if err := gdc.LoadCache(); err == nil {
			// Verify essential fields from loaded cache to ensure it's not corrupted/empty.
			if gdc.Data.Metadata.Repo.Commit.SHA == "" || gdc.Data.Metadata.Collector.DateCollected.IsZero() {
				fmt.Println("Cache seems incomplete or corrupted. Re-collecting.")
			} else {
				return nil // Successfully loaded from cache
			}
		} else {
			fmt.Printf("Failed to load cache: %v. Re-collecting.\n", err)
		}
	}

	fmt.Println("No valid cache found or cache load failed. Collecting data from repository...")
	if err := gdc.collectMetadata(); err != nil {
		return fmt.Errorf("failed to collect metadata: %w", err)
	}

	// Print progress (simple version)
	fmt.Println("Processing commits...")
	commits, err := gitutil.IterateCommits(gdc.repo, gdc.head)
	if err != nil {
		return fmt.Errorf("failed to iterate commits: %w", err)
	}

	for _, commit := range commits {
		if err := gdc.collectCommitData(commit); err != nil {
			// Log error but continue processing other commits
			fmt.Printf("Warning: failed to process commit %s: %v\n", commit.Hash.String(), err)
		}
	}
	
	fmt.Println("Processing file blames...")
	if err := gdc.collectBlameDataByFile(); err != nil {
		return fmt.Errorf("failed to collect blame data: %w", err)
	}

	fmt.Println("Aggregating contributor line counts...")
	gdc.collectActiveLineCountByContributor()
	
	fmt.Println("Data collection complete.")
	if err := gdc.SaveCache(); err != nil {
		return fmt.Errorf("failed to save data to cache: %w", err)
	}

	return nil
}

func (gdc *GitDataCollector) collectMetadata() error {
	currentUser, err := user.Current()
	userName := "unknown"
	if err == nil {
		userName = currentUser.Username
	}

	hostname, _ := os.Hostname()
	gitVersion, _ := gitutil.GetGitVersion() // Pure Go, so not system git version

	remoteURL, err := gitutil.GetRepoRemoteURL(gdc.repo)
	if err != nil {
		fmt.Printf("Warning: could not get remote URL: %v\n", err)
		remoteURL = "unknown"
	}
	
	branchName, err := gitutil.GetRepoBranch(gdc.repo, gdc.head)
	if err != nil {
		fmt.Printf("Warning: could not get branch name: %v\n", err)
		// Use HEAD SHA if branch detection failed
		branchName = gdc.head.Hash.String() + " (error determining branch)"
	}


	gdc.Data.Metadata = models.Metadata{
		Collector: models.CollectorMetadata{
			InquisitorVersion: InquisitorVersion,
			DateCollected:     time.Now().UTC(),
			User:              userName,
			Hostname:          hostname,
			Platform:          fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			GoVersion:         runtime.Version(),
			GitVersion:        gitVersion,
		},
		Repo: models.RepoMetadata{
			URL:    remoteURL,
			Branch: branchName,
			Commit: gitutil.GetCommitDetails(gdc.head),
		},
	}
	return nil
}

func (gdc *GitDataCollector) collectCommitData(commit *object.Commit) error {
	// 1. Collect data for contributor stats
	committerName := strings.TrimSpace(strings.Split(commit.Committer.Name, "<")[0])
	committerEmail := commit.Committer.Email

	if _, ok := gdc.Data.Contributors[committerName]; !ok {
		gdc.Data.Contributors[committerName] = models.Contributor{
			Identities:   []string{},
			CommitCount:  0,
			Insertions:   0,
			Deletions:    0,
			ActiveLines:  0, // Calculated later
		}
	}
	contribData := gdc.Data.Contributors[committerName] // Get a copy
	
	isNewIdentity := true
	for _, identity := range contribData.Identities {
		if identity == committerEmail {
			isNewIdentity = false
			break
		}
	}
	if isNewIdentity {
		contribData.Identities = append(contribData.Identities, committerEmail)
	}

	contribData.CommitCount++

	// Get stats for this commit
	insertions, deletions, filesChangedMap, err := gitutil.GetCommitStats(commit)
	if err != nil {
		return fmt.Errorf("failed to get stats for commit %s: %w", commit.Hash.String(), err)
	}
	contribData.Insertions += insertions
	contribData.Deletions += deletions
	gdc.Data.Contributors[committerName] = contribData // Put the modified copy back

	// 2. Collect data for history log
	var parentSHAs []string
	for i := 0; i < commit.NumParents(); i++ {
		parent, errParent := commit.Parent(i)
		if errParent == nil {
			parentSHAs = append(parentSHAs, parent.Hash.String())
		}
	}

	historyItem := models.CommitHistoryItem{
		Commit:      commit.Hash.String(),
		Parents:     parentSHAs,
		Tree:        commit.TreeHash.String(),
		Contributor: fmt.Sprintf("%s (%s)", commit.Committer.Name, commit.Committer.Email),
		Date:        commit.Committer.When,
		Message:     commit.Message, // Full message for history
		Insertions:  insertions,
		Deletions:   deletions,
		FilesChanged: filesChangedMap,
	}
	gdc.Data.History = append(gdc.Data.History, historyItem)
	return nil
}

func (gdc *GitDataCollector) collectBlameDataByFile() error {
	// Get list of files at HEAD
	filePaths, err := gitutil.GetFilePaths(gdc.repo, gdc.head)
	if err != nil {
		return fmt.Errorf("failed to list files at HEAD: %w", err)
	}

	numFiles := len(filePaths)
	if numFiles == 0 {
		return nil
	}

	// Worker pool setup
	numWorkers := runtime.NumCPU()
	if numFiles < numWorkers {
		numWorkers = numFiles // Don't start more workers than files
	}

	jobs := make(chan string, numFiles)
	results := make(chan struct {
		Path string
		Stats *models.FileBlameStats
		Err error
	}, numFiles)

	var wg sync.WaitGroup // Use sync.WaitGroup

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			// fmt.Printf("Worker %d started\n", workerID)
			for filePath := range jobs {
				// fmt.Printf("Worker %d processing %s\n", workerID, filePath)
				blameStats, errBlame := gitutil.GetBlameForFile(gdc.repo, gdc.head, filePath)
				results <- struct {
					Path string
					Stats *models.FileBlameStats
					Err error
				}{Path: filePath, Stats: blameStats, Err: errBlame}
			}
			// fmt.Printf("Worker %d finished\n", workerID)
		}(w)
	}

	// Distribute jobs
	for _, filePath := range filePaths {
		jobs <- filePath
	}
	close(jobs) // Signal workers that no more jobs will be sent

	// Collect results
	// It's important to wait for all workers to finish *before* closing the results channel.
	// The easiest way to manage this is to know how many results to expect.
	
	fmt.Println("Waiting for file blame processing to complete...")
	
	// Wait for all workers to complete in a separate goroutine
	// so that we don't block collecting results if a worker goroutine panics.
	go func() {
		wg.Wait()
		close(results) // Now it's safe to close results channel
		// fmt.Println("All workers done, results channel closed.")
	}()

	processedCount := 0
	for result := range results {
		processedCount++
		fmt.Printf("Processed file %d/%d: %s\n", processedCount, numFiles, result.Path)
		if result.Err != nil {
			fmt.Printf("Warning: could not get blame for file %s: %v\n", result.Path, result.Err)
			continue
		}
		if result.Stats != nil && result.Stats.TotalLines > 0 {
			gdc.Data.Files[result.Path] = models.FileData{
				DateIntroduced:     result.Stats.DateIntroduced,
				OriginalAuthor:     result.Stats.OriginalAuthor,
				TotalCommits:       result.Stats.TotalCommits,
				TotalLines:         result.Stats.TotalLines,
				TopContributor:     result.Stats.TopContributor,
				LinesByContributor: result.Stats.LinesByContributor,
			}
		}
	}
	// fmt.Println("Finished collecting all blame results.")
	return nil
}

func (gdc *GitDataCollector) collectActiveLineCountByContributor() {
	// This part needs to be thread-safe if accessed concurrently, but it's called sequentially after all file data is collected.
	// However, gdc.Data.Contributors is modified. If other parts were concurrent and also modified it,
	// this would need a mutex or to operate on a local copy and then update.
	// For now, it's safe as it's called after the concurrent collectBlameDataByFile has finished and results are aggregated.

	for contributorName := range gdc.Data.Contributors {
		// It's safer to get a fresh copy of contributorData if map values are structs
		// and we are modifying them, especially if concurrency was involved earlier.
		// However, here we just calculate activeLines and then update the map entry.
		contributorData := gdc.Data.Contributors[contributorName]
		activeLines := 0
		for _, fileData := range gdc.Data.Files { // gdc.Data.Files is fully populated by now
			if lines, ok := fileData.LinesByContributor[contributorName]; ok {
				activeLines += lines
			}
		}
		contributorData.ActiveLines = activeLines
		gdc.Data.Contributors[contributorName] = contributorData // Update the map with new ActiveLines
	}
}

// ClearCache removes the cache file for the current HEAD commit.
func (gdc *GitDataCollector) ClearCache() error {
	cacheFile := gdc.cachePath()
	err := os.Remove(cacheFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file %s: %w", cacheFile, err)
	}
	if err == nil {
		fmt.Printf("Cache file %s removed successfully.\n", cacheFile)
	}
	return nil
}

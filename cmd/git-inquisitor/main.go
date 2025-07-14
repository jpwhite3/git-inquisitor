package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/user/git-inquisitor-go/internal/collector"
	"github.com/user/git-inquisitor-go/internal/report"
)

var (
	// Used for flags.
	outputFilePath string

	rootCmd = &cobra.Command{
		Use:   "git-inquisitor",
		Short: "Git Inquisitor is a git repository analysis tool.",
		Long: `A tool designed to provide teams with useful information about a 
git repository and its contributors. It provides history details, 
file level contribution statistics, and contributor level statistics.`,
	}

	collectCmd = &cobra.Command{
		Use:   "collect [REPO_PATH]",
		Short: "Collects data from a git repository and caches it.",
		Long:  `Scans a git repository located at REPO_PATH, collects various metrics and statistics, and caches the results for later reporting.`,
		Args:  cobra.ExactArgs(1), // Requires exactly one argument: repo-path
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := args[0]
			absRepoPath, err := filepath.Abs(repoPath)
			if err != nil {
				return fmt.Errorf("error getting absolute path for '%s': %w", repoPath, err)
			}

			// Check if repoPath is a directory and looks like a git repo
			stat, err := os.Stat(absRepoPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("repository path '%s' does not exist", absRepoPath)
				}
				return fmt.Errorf("error accessing repository path '%s': %w", absRepoPath, err)
			}
			if !stat.IsDir() {
				return fmt.Errorf("repository path '%s' is not a directory", absRepoPath)
			}
			// Basic check for .git directory
			if _, err := os.Stat(filepath.Join(absRepoPath, ".git")); os.IsNotExist(err) {
                 // Could also be a bare repo, where absRepoPath itself is .git, or has HEAD file
                if _, errHead := os.Stat(filepath.Join(absRepoPath, "HEAD")); os.IsNotExist(errHead) {
                    return fmt.Errorf("'%s' does not appear to be a git repository (missing .git directory or HEAD file)", absRepoPath)
                }
			}


			fmt.Printf("Collecting data for repository: %s\n", absRepoPath)
			col, err := collector.NewGitDataCollector(absRepoPath)
			if err != nil {
				return fmt.Errorf("failed to initialize collector for %s: %w", absRepoPath, err)
			}

			if err := col.Collect(); err != nil {
				return fmt.Errorf("error during data collection for %s: %w", absRepoPath, err)
			}
			fmt.Println("Data collection successful.")
			// Consider option to clear cache
			// clearCache, _ := cmd.Flags().GetBool("clear-cache")
			// if clearCache {
			// 	fmt.Println("Clearing cache...")
			//  if err := col.ClearCache(); err != nil {
			//      fmt.Fprintf(os.Stderr, "Error clearing cache: %v\n", err)
			//  }
			// }
			return nil
		},
	}

	reportCmd = &cobra.Command{
		Use:   "report [REPO_PATH] [html|json]",
		Short: "Generates a report from collected data.",
		Long: `Generates a report in the specified format (html or json) using previously 
collected data for the git repository at REPO_PATH.`,
		Args: cobra.ExactArgs(2), // Requires repo-path and report-format
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := args[0]
			reportFormat := args[1]

			absRepoPath, err := filepath.Abs(repoPath)
			if err != nil {
				return fmt.Errorf("error getting absolute path for '%s': %w", repoPath, err)
			}
			
			// Validate report format
			if reportFormat != "html" && reportFormat != "json" {
				return fmt.Errorf("invalid report format '%s'. Must be 'html' or 'json'", reportFormat)
			}

			// Determine output file path
			if outputFilePath == "" {
				outputFilePath = fmt.Sprintf("inquisitor-report.%s", reportFormat)
			}
			absOutputFilePath, err := filepath.Abs(outputFilePath)
			if err != nil {
				return fmt.Errorf("invalid output file path '%s': %w", outputFilePath, err)
			}


			fmt.Printf("Generating %s report for repository: %s\n", reportFormat, absRepoPath)
			col, err := collector.NewGitDataCollector(absRepoPath)
			if err != nil {
				return fmt.Errorf("failed to initialize collector for %s: %w", absRepoPath, err)
			}

			// Load data - Collect() will try cache first, then collect if needed.
			// This matches Python version's behavior where report implies collection if no cache.
			if err := col.Collect(); err != nil {
				// If collection fails (e.g. repo disappeared after initial collect command), report should fail.
				return fmt.Errorf("failed to load or collect data for %s: %w", absRepoPath, err)
			}
			
			var adapter report.ReportAdapter
			if reportFormat == "html" {
				adapter = &report.HtmlReportAdapter{}
			} else { // reportFormat == "json"
				adapter = &report.JsonReportAdapter{}
			}

			fmt.Println("Preparing report data...")
			if err := adapter.PrepareData(&col.Data); err != nil {
				return fmt.Errorf("failed to prepare %s report data: %w", reportFormat, err)
			}

			fmt.Printf("Writing report to: %s\n", absOutputFilePath)
			if err := adapter.Write(absOutputFilePath); err != nil {
				return fmt.Errorf("failed to write %s report to %s: %w", reportFormat, absOutputFilePath, err)
			}

			fmt.Printf("%s report generated successfully: %s\n", strings.ToUpper(reportFormat), absOutputFilePath)
			return nil
		},
	}
)

func init() {
	// Add flags to reportCmd
	reportCmd.Flags().StringVarP(&outputFilePath, "output-file-path", "o", "", "Output file path for the report")
	// Example for adding a flag to collectCmd if needed later:
	// collectCmd.Flags().Bool("clear-cache", false, "Clears existing cache before collecting")


	// Add subcommands to rootCmd
	rootCmd.AddCommand(collectCmd)
	rootCmd.AddCommand(reportCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

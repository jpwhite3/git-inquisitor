// Package chart provides functionality for chart data structures used in reports.
// Note: Static chart generation has been removed in favor of Chart.js for interactive charts.
package chart

import (
	"testing"

	"github.com/user/git-inquisitor-go/internal/models"
)

// TestPopulateHTMLChartData tests that the PopulateHTMLChartData function
// returns an empty HTMLChartData struct as expected, since chart generation
// has been moved to Chart.js in the HTML template.
func TestPopulateHTMLChartData(t *testing.T) {
	// Create test data
	data := &models.CollectedData{
		Contributors: map[string]models.Contributor{
			"User A": {CommitCount: 10, Insertions: 100, Deletions: 50},
		},
	}
	
	// Test with normal data
	htmlCharts, err := PopulateHTMLChartData(data)
	
	// Verify no error is returned
	if err != nil {
		t.Errorf("PopulateHTMLChartData() returned error: %v", err)
	}

	// Verify all fields are empty as expected since we're now using Chart.js
	expectedEmptyStruct := HTMLChartData{}
	if htmlCharts != expectedEmptyStruct {
		t.Errorf("PopulateHTMLChartData() expected empty struct, got: %+v", htmlCharts)
	}

	// Test with empty data to ensure graceful handling
	emptyData := &models.CollectedData{
		Contributors: map[string]models.Contributor{},
	}
	emptyCharts, err := PopulateHTMLChartData(emptyData)
	
	// Verify no error is returned
	if err != nil {
		t.Errorf("PopulateHTMLChartData() with empty data returned error: %v", err)
	}
	
	// Verify all fields are empty
	if emptyCharts != expectedEmptyStruct {
		t.Errorf("PopulateHTMLChartData() with empty data expected empty struct, got: %+v", emptyCharts)
	}
}

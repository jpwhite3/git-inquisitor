package chart

import (
	"log"

	"github.com/user/git-inquisitor-go/internal/models"
)

// Struct to hold data for charts that might be used by HTML report
// This is kept for backward compatibility but is no longer used for static images
// as we're now using Chart.js for interactive charts
type HTMLChartData struct {
	CommitsByAuthorChart string
	ChangesByAuthorChart string
	CommitHistoryChart   string
	ChangeHistoryChart   string
}

// PopulateHTMLChartData returns an empty HTMLChartData struct
// as we're now using Chart.js for interactive charts directly in the template
func PopulateHTMLChartData(_ *models.CollectedData) (HTMLChartData, error) {
	log.Println("Using Chart.js for interactive charts instead of static images")
	return HTMLChartData{}, nil
}

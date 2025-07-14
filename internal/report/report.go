package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/user/git-inquisitor-go/internal/chart"
	"github.com/user/git-inquisitor-go/internal/models"
	// To use humanize functions like in Jinja template, we might need a library
	// or implement them. For now, I'll skip complex humanize filters.
	// Example: "github.com/dustin/go-humanize"
)

// ReportAdapter defines the interface for generating different report formats.
type ReportAdapter interface {
	PrepareData(data *models.CollectedData) error
	Write(outputFilePath string) error
}

// --- JSON Report Adapter ---

// JsonReportAdapter generates reports in JSON format.
type JsonReportAdapter struct {
	reportData string
}

// PrepareData marshals the collected data into a JSON string.
func (jra *JsonReportAdapter) PrepareData(data *models.CollectedData) error {
	jsonData, err := json.MarshalIndent(data, "", "  ") // Use indent for readability
	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %w", err)
	}
	jra.reportData = string(jsonData)
	return nil
}

// Write saves the JSON report data to the specified output file.
func (jra *JsonReportAdapter) Write(outputFilePath string) error {
	if err := os.MkdirAll(filepath.Dir(outputFilePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for report file %s: %w", outputFilePath, err)
	}
	return os.WriteFile(outputFilePath, []byte(jra.reportData), 0644)
}

// --- HTML Report Adapter ---

// HtmlReportAdapter generates reports in HTML format.
type HtmlReportAdapter struct {
	rawDatarawData *models.CollectedData
	chartData  chart.HTMLChartData
	reportBuf  bytes.Buffer // To store rendered HTML
}

// PrepareData prepares data for HTML report, including generating charts.
func (hra *HtmlReportAdapter) PrepareData(data *models.CollectedData) error {
	hra.rawDatarawData = data

	// Sort history by date (descending, newest first) as in Python version for display
	// The collector already sorts it ascending (oldest first) for processing.
	// For display, often newest first is preferred in history tables.
	sort.SliceStable(hra.rawDatarawData.History, func(i, j int) bool {
		return hra.rawDatarawData.History[i].Date.After(hra.rawDatarawData.History[j].Date)
	})
	
	// Sort files by path for consistent display
	// This wasn't explicitly done in Python but is good practice for Go maps.
	// However, the template ranges over `data.files` which is a map, so order isn't guaranteed
	// unless we convert it to a slice of structs or sort keys.
	// For now, we'll rely on template's range behavior.

	// Generate chart data
	charts, err := chart.PopulateHTMLChartData(data)
	if err != nil {
		// Log the error but attempt to generate report without charts or with partial charts
		fmt.Printf("Warning: Error generating chart data: %v. HTML report may be incomplete.\n", err)
		// Initialize charts struct to avoid nil pointer if some charts failed
		hra.chartData = chart.HTMLChartData{} 
	} else {
		hra.chartData = charts
	}
	

	// Define template functions (Go equivalent of Jinja filters/globals)
	funcMap := template.FuncMap{
		"ToUpper":      strings.ToUpper,
		"Capitalize":   strings.Title, // Note: strings.Title is deprecated, consider cases.Title
		"Replace":      strings.ReplaceAll,
		"Truncate": func(s string, length int, killwords bool, end string) string { // Basic truncate
			if len(s) <= length {
				return s
			}
			if !killwords {
				// find last space within length
				if idx := strings.LastIndex(s[:length], " "); idx != -1 {
					return s[:idx] + end
				}
			}
			return s[:length-len(end)] + end
		},
		"FormatDateTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05 MST")
		},
		"FormatDate": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
		"ShortSha": func(sha string) string {
			if len(sha) > 8 {
				return sha[:8]
			}
			return sha
		},
		"CommitterName": func(contributor string) string {
			parts := strings.Split(contributor, " (")
			return parts[0]
		},
		"CommitMsgShort": func(msg string) string {
			lines := strings.Split(msg, "\n")
			return lines[0] // First line as short message
		},
		"Len": func(item interface{}) int { // Generic length for slices/maps in template
            switch v := item.(type) {
            case []models.CommitHistoryItem: // Be specific if needed for type safety
                return len(v)
            case map[string]models.FileCommitStats:
                return len(v)
            case string:
                return len(v)
            // Add other types as necessary
            default:
                return 0
            }
        },
		// Add humanize functions if a library is chosen.
		// For now, they will be missing from the template or need to be removed from it.
		// "HumanizeMetric": func ...
	}

	// Load and parse the HTML template
	// The template path needs to be relative to the running binary or use embed.
	// For now, assume it's in a known relative path "templates/report.html.template"
	// This path might need adjustment based on how the application is run/deployed.
	tmplPath := filepath.Join("templates", "report.html.template")
	// Check if template exists
	if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
		// Fallback for tests or different execution contexts
		// This is a common issue with finding templates.
		// Using go:embed is a more robust solution for production.
		altPath := filepath.Join("..", "templates", "report.html.template") // If running from cmd/
		if _, altErr := os.Stat(altPath); !os.IsNotExist(altErr) {
			tmplPath = altPath
		} else {
			// One more common pattern when tests are in subdirs
			altPath2 := filepath.Join("..", "..", "templates", "report.html.template")
			if _, altErr2 := os.Stat(altPath2); !os.IsNotExist(altErr2) {
				tmplPath = altPath2
			} else {
				return fmt.Errorf("HTML template not found at %s or fallback paths: %w", tmplPath, err)
			}
		}
	}


	tmpl, err := template.New(filepath.Base(tmplPath)).Funcs(funcMap).ParseFiles(tmplPath)
	if err != nil {
		return fmt.Errorf("failed to parse HTML template %s: %w", tmplPath, err)
	}

	templateData := struct {
		Data      *models.CollectedData
		ChartData chart.HTMLChartData
	}{
		Data:      hra.rawDatarawData,
		ChartData: hra.chartData,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		return fmt.Errorf("failed to execute HTML template: %w", err)
	}
	hra.reportBuf = buf
	return nil
}

// Write saves the HTML report data to the specified output file.
func (hra *HtmlReportAdapter) Write(outputFilePath string) error {
	if err := os.MkdirAll(filepath.Dir(outputFilePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for HTML report file %s: %w", outputFilePath, err)
	}
	return os.WriteFile(outputFilePath, hra.reportBuf.Bytes(), 0644)
}

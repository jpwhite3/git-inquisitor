package chart

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/color"
	"image/png"
	"log"
	"sort"
	"time"

	"github.com/user/git-inquisitor-go/internal/models"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// Struct to hold data for charts that might be used by HTML report
type HTMLChartData struct {
	CommitsByAuthorChart string
	ChangesByAuthorChart string
	CommitHistoryChart   string
	ChangeHistoryChart   string
}

func generatePlotImageBase64(p *plot.Plot) (string, error) {
	// Set default font to avoid errors if system fonts are not found
	plot.DefaultFont = draw.Font{Typeface: "Liberation", Variant: "Sans"}
	plotter.DefaultFont = draw.Font{Typeface: "Liberation", Variant: "Sans"}


	// Create a writer to capture the PNG output.
	writer, err := p.WriterTo(4*vg.Inch, 4*vg.Inch, "png") // Default size, can be adjusted
	if err != nil {
		return "", fmt.Errorf("failed to create plot writer: %w", err)
	}

	var buf bytes.Buffer
	if _, err := writer.WriteTo(&buf); err != nil {
		return "", fmt.Errorf("failed to write plot to buffer: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}


// GeneratePieChart creates a pie chart and returns it as a base64 encoded PNG string.
func GeneratePieChart(dataMap map[string]float64, title string) (string, error) {
	p := plot.New()
	p.Title.Text = title
	// p.HideAxes() // Pie charts don't typically have axes shown

	var values plotter.Values
	var labels []string // Keep labels in sync with values

	// Sort map keys for consistent chart generation (optional, but good for testing)
	keys := make([]string, 0, len(dataMap))
	for k := range dataMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if dataMap[k] > 0 { // Pie charts usually only show positive values
			values = append(values, dataMap[k])
			labels = append(labels, fmt.Sprintf("%s (%.1f)", k, dataMap[k])) // Include value in label for clarity
		}
	}
    
	if len(values) == 0 {
		return "", fmt.Errorf("no data to plot for pie chart: %s", title)
	}

	pie, err := plotter.NewPieChart(values)
	if err != nil {
		return "", fmt.Errorf("failed to create pie chart for %s: %w", title, err)
	}
	
	// Customize pie chart appearance
	pie.Labels.Nominal = labels
	pie.Labels.Values.Show = true // Show percentage values on slices
	// pie.Explode = plotter.Values{0.05} // Slightly explode the first slice (optional)

	p.Add(pie)

	return generatePlotImageBase64(p)
}


// GenerateCommitActivityChart creates a line chart for commit activity over time.
func GenerateCommitActivityChart(history []models.CommitHistoryItem, title string) (string, error) {
	p := plot.New()
	p.Title.Text = title
	p.X.Label.Text = "Date"
	p.Y.Label.Text = "Commits"
	p.X.Tick.Marker = plot.TimeTicks{Format: "2006-01-02"} // Date format for X-axis

	// Aggregate commits by date
	commitsByDate := make(map[time.Time]int)
	for _, item := range history {
		// Normalize to day
		day := item.Date.Truncate(24 * time.Hour)
		commitsByDate[day]++
	}

	if len(commitsByDate) == 0 {
		return "", fmt.Errorf("no commit history data to plot for: %s", title)
	}
	
	pts := make(plotter.XYs, 0, len(commitsByDate))
	for date, count := range commitsByDate {
		pts = append(pts, plotter.XY{X: float64(date.Unix()), Y: float64(count)})
	}

	// Sort points by date for correct line plotting
	sort.Slice(pts, func(i, j int) bool {
		return pts[i].X < pts[j].X
	})

	line, err := plotter.NewLine(pts)
	if err != nil {
		return "", fmt.Errorf("failed to create new line for %s: %w", title, err)
	}
	line.Color = color.RGBA{B: 255, A: 255} // Blue line
	p.Add(line)

	// Add a scatter plot for points to make them more visible
	// scatter, err := plotter.NewScatter(pts)
	// if err != nil {
	// 	return "", fmt.Errorf("failed to create scatter plot for %s: %w", title, err)
	// }
	// scatter.GlyphStyle.Radius = vg.Points(2)
	// p.Add(scatter)
	
	p.X.Tick.Marker = plot.TimeTicks{Format: "2006-01-02"}
	p.X.Label.Text = "Date"
	p.Y.Label.Text = "Number of Commits"
	p.Add(plotter.NewGrid())


	return generatePlotImageBase64(p)
}

// GenerateLineChangeChart creates a line chart for insertions and deletions over time.
func GenerateLineChangeChart(history []models.CommitHistoryItem, title string) (string, error) {
	p := plot.New()
	p.Title.Text = title
	p.X.Label.Text = "Date"
	p.Y.Label.Text = "Lines Changed"
	p.X.Tick.Marker = plot.TimeTicks{Format: "2006-01-02"}
	p.Legend.Top = true

	insertionsByDate := make(map[time.Time]int)
	deletionsByDate := make(map[time.Time]int)

	for _, item := range history {
		day := item.Date.Truncate(24 * time.Hour)
		insertionsByDate[day] += item.Insertions
		deletionsByDate[day] += item.Deletions 
	}

	if len(insertionsByDate) == 0 && len(deletionsByDate) == 0 {
		return "", fmt.Errorf("no line change data to plot for: %s", title)
	}

	ptsInsertions := make(plotter.XYs, 0, len(insertionsByDate))
	for date, count := range insertionsByDate {
		ptsInsertions = append(ptsInsertions, plotter.XY{X: float64(date.Unix()), Y: float64(count)})
	}
	sort.Slice(ptsInsertions, func(i, j int) bool { return ptsInsertions[i].X < ptsInsertions[j].X })

	ptsDeletions := make(plotter.XYs, 0, len(deletionsByDate))
	for date, count := range deletionsByDate {
		// Plot deletions as positive numbers on the graph, but they represent removed lines.
		// Or, if you want them below X-axis, this needs more complex Y-axis handling.
		// For simplicity, like GitHub, often both are positive.
		ptsDeletions = append(ptsDeletions, plotter.XY{X: float64(date.Unix()), Y: float64(count)})
	}
	sort.Slice(ptsDeletions, func(i, j int) bool { return ptsDeletions[i].X < ptsDeletions[j].X })

	if len(ptsInsertions) > 0 {
		lineInsertions, err := plotter.NewLine(ptsInsertions)
		if err != nil {
			log.Printf("Warning: failed to create insertions line for %s: %v", title, err)
		} else {
			lineInsertions.Color = color.RGBA{G: 200, A: 255} // Green line
			lineInsertions.LineStyle.Width = vg.Points(2)
			p.Add(lineInsertions)
			p.Legend.Add("Insertions", lineInsertions)
		}
	}

	if len(ptsDeletions) > 0 {
		lineDeletions, err := plotter.NewLine(ptsDeletions)
		if err != nil {
			log.Printf("Warning: failed to create deletions line for %s: %v", title, err)
		} else {
			lineDeletions.Color = color.RGBA{R: 200, A: 255} // Red line
			lineDeletions.LineStyle.Width = vg.Points(2)
			p.Add(lineDeletions)
			p.Legend.Add("Deletions", lineDeletions)
		}
	}
	
	p.X.Tick.Marker = plot.TimeTicks{Format: "2006-01-02"}
	p.X.Label.Text = "Date"
	p.Y.Label.Text = "Lines"
	p.Add(plotter.NewGrid())


	return generatePlotImageBase64(p)
}

// GenerateCharts populates the HTMLChartData struct with base64 encoded chart images.
func PopulateHTMLChartData(data *models.CollectedData) (HTMLChartData, error) {
	var charts HTMLChartData
	var err error

	// 1. Commits by Author Pie Chart
	commitsMap := make(map[string]float64)
	for name, cData := range data.Contributors {
		commitsMap[name] = float64(cData.CommitCount)
	}
	charts.CommitsByAuthorChart, err = GeneratePieChart(commitsMap, "Commits by Author")
	if err != nil {
		log.Printf("Warning: Failed to generate 'Commits by Author' chart: %v", err)
		// Allow continuing if one chart fails, or return err immediately
	}

	// 2. Line Changes by Author Pie Chart
	changesMap := make(map[string]float64)
	for name, cData := range data.Contributors {
		changesMap[name] = float64(cData.Insertions + cData.Deletions)
	}
	charts.ChangesByAuthorChart, err = GeneratePieChart(changesMap, "Line Changes by Author")
	if err != nil {
		log.Printf("Warning: Failed to generate 'Line Changes by Author' chart: %v", err)
	}

	// 3. Commit History Chart (Commits over Time)
	charts.CommitHistoryChart, err = GenerateCommitActivityChart(data.History, "Commit Activity Over Time")
	if err != nil {
		log.Printf("Warning: Failed to generate 'Commit Activity' chart: %v", err)
	}

	// 4. Change History Chart (Line Changes over Time)
	charts.ChangeHistoryChart, err = GenerateLineChangeChart(data.History, "Line Changes Over Time")
	if err != nil {
		log.Printf("Warning: Failed to generate 'Line Changes' chart: %v", err)
	}
	
	// If any chart generation failed and strict error handling is desired, check errors here.
	// For now, we log warnings and proceed with potentially missing charts.

	return charts, nil // Return nil error, relying on logs for individual chart failures
}

// Package components provides reusable TUI components for conference talk ranking.
// This file implements a progress indicators component for convergence tracking
// with real-time progress bars and metrics display for Elo algorithm optimization.
package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/pashagolub/confelo/pkg/data"
	"github.com/pashagolub/confelo/pkg/elo"
)

// Progress displays real-time convergence tracking and completion estimates
type Progress struct {
	// UI components
	container      *tview.Flex
	coverageBar    *tview.TextView
	convergenceBar *tview.TextView
	metricsText    *tview.TextView
	statusText     *tview.TextView

	// Data and state
	lastUpdate     time.Time
	engine         *elo.Engine
	history        *elo.ComparisonHistory
	config         elo.OptimizationConfig
	currentMetrics *elo.ProgressMetrics

	// Display configuration
	showBars       bool
	showMetrics    bool
	showEstimates  bool
	updateInterval time.Duration

	// Colors
	progressColor tcell.Color
	completeColor tcell.Color
	textColor     tcell.Color
	borderColor   tcell.Color

	// Update callbacks
	onUpdate    func(metrics *elo.ProgressMetrics)
	onConverged func(confidence float64)
}

// ProgressConfig holds configuration options for the progress indicator
type ProgressConfig struct {
	ShowBars       bool
	ShowMetrics    bool
	ShowEstimates  bool
	UpdateInterval time.Duration
	ProgressColor  tcell.Color
	CompleteColor  tcell.Color
	TextColor      tcell.Color
	BorderColor    tcell.Color
	OnUpdate       func(metrics *elo.ProgressMetrics)
	OnConverged    func(confidence float64)
}

// NewProgress creates a new progress indicator component
func NewProgress(engine *elo.Engine, config ProgressConfig) *Progress {
	p := &Progress{
		container:      tview.NewFlex(),
		coverageBar:    tview.NewTextView(),
		convergenceBar: tview.NewTextView(),
		metricsText:    tview.NewTextView(),
		statusText:     tview.NewTextView(),
		lastUpdate:     time.Now(),
		engine:         engine,
		config:         elo.DefaultOptimizationConfig(),
		showBars:       config.ShowBars,
		showMetrics:    config.ShowMetrics,
		showEstimates:  config.ShowEstimates,
		updateInterval: config.UpdateInterval,
		progressColor:  config.ProgressColor,
		completeColor:  config.CompleteColor,
		textColor:      config.TextColor,
		borderColor:    config.BorderColor,
		onUpdate:       config.OnUpdate,
		onConverged:    config.OnConverged,
	}

	// Set default values if not specified
	if p.updateInterval == 0 {
		p.updateInterval = 100 * time.Millisecond
	}
	if p.progressColor == 0 {
		p.progressColor = tcell.ColorBlue
	}
	if p.completeColor == 0 {
		p.completeColor = tcell.ColorGreen
	}
	if p.textColor == 0 {
		p.textColor = tcell.ColorWhite
	}
	if p.borderColor == 0 {
		p.borderColor = tcell.ColorDarkGray
	}

	p.initializeUI()
	return p
}

// DefaultProgressConfig returns sensible defaults for progress indicators
func DefaultProgressConfig() ProgressConfig {
	return ProgressConfig{
		ShowBars:       true,
		ShowMetrics:    true,
		ShowEstimates:  true,
		UpdateInterval: 100 * time.Millisecond,
		ProgressColor:  tcell.ColorBlue,
		CompleteColor:  tcell.ColorGreen,
		TextColor:      tcell.ColorWhite,
		BorderColor:    tcell.ColorDarkGray,
	}
}

// initializeUI sets up the progress indicator layout and styling
func (p *Progress) initializeUI() {
	// Configure progress bars (using TextViews for custom progress display)
	p.coverageBar.SetBorder(true).SetTitle("Coverage Progress")
	p.coverageBar.SetBorderColor(p.borderColor)
	p.coverageBar.SetTextColor(p.textColor)
	p.coverageBar.SetDynamicColors(true)
	p.coverageBar.SetTextAlign(tview.AlignCenter)

	p.convergenceBar.SetBorder(true).SetTitle("Convergence Progress")
	p.convergenceBar.SetBorderColor(p.borderColor)
	p.convergenceBar.SetTextColor(p.textColor)
	p.convergenceBar.SetDynamicColors(true)
	p.convergenceBar.SetTextAlign(tview.AlignCenter)

	// Configure text views
	p.metricsText.SetBorder(true).SetTitle("Metrics")
	p.metricsText.SetBorderColor(p.borderColor)
	p.metricsText.SetTextColor(p.textColor)
	p.metricsText.SetDynamicColors(true)

	p.statusText.SetBorder(true).SetTitle("Status")
	p.statusText.SetBorderColor(p.borderColor)
	p.statusText.SetTextColor(p.textColor)
	p.statusText.SetDynamicColors(true)

	// Layout components
	p.container.SetDirection(tview.FlexRow)

	if p.showBars {
		// Progress bars section
		barsContainer := tview.NewFlex().SetDirection(tview.FlexColumn)
		barsContainer.AddItem(p.coverageBar, 0, 1, false)
		barsContainer.AddItem(p.convergenceBar, 0, 1, false)
		p.container.AddItem(barsContainer, 6, 0, false)
	}

	if p.showMetrics || p.showEstimates {
		// Text information section
		textContainer := tview.NewFlex().SetDirection(tview.FlexColumn)

		if p.showMetrics {
			textContainer.AddItem(p.metricsText, 0, 1, false)
		}

		if p.showEstimates {
			textContainer.AddItem(p.statusText, 0, 1, false)
		}

		p.container.AddItem(textContainer, 0, 1, false)
	}
}

// Update refreshes the progress indicators with current data
func (p *Progress) Update(proposals []data.Proposal, history *elo.ComparisonHistory) {
	// Throttle updates to avoid excessive redraws
	if time.Since(p.lastUpdate) < p.updateInterval {
		return
	}
	p.lastUpdate = time.Now()

	// Convert proposals to ratings for engine
	ratings := make([]elo.Rating, len(proposals))
	for i, proposal := range proposals {
		ratings[i] = elo.Rating{
			ID:         proposal.ID,
			Score:      proposal.Score,
			Confidence: 0.0, // Will be calculated by engine
			Games:      0,   // Will be calculated by engine
		}
	}

	// Get current progress metrics from engine
	p.currentMetrics = p.engine.GetProgressMetrics(ratings, history, p.config)
	p.history = history

	// Update progress bars
	if p.showBars {
		p.updateProgressBars()
	}

	// Update text displays
	if p.showMetrics {
		p.updateMetricsDisplay()
	}

	if p.showEstimates {
		p.updateStatusDisplay()
	}

	// Check for convergence and call callback
	if p.onConverged != nil && p.isConverged() {
		avgConfidence := p.calculateAverageConfidence()
		p.onConverged(avgConfidence)
	}

	// Call update callback if provided
	if p.onUpdate != nil {
		p.onUpdate(p.currentMetrics)
	}
}

// updateProgressBars refreshes the progress bar displays
func (p *Progress) updateProgressBars() {
	if p.currentMetrics == nil {
		return
	}

	// Update coverage progress bar
	coverage := p.currentMetrics.CoverageComplete
	coverageText := p.createProgressBar(coverage, coverage >= 0.95)
	coverageText += fmt.Sprintf("\n[white]%.1f%% Complete", coverage*100)
	p.coverageBar.SetText(coverageText)

	// Update convergence progress bar (based on convergence rate)
	// Lower convergence rate = higher progress toward stability
	convergenceProgress := 1.0 - p.currentMetrics.ConvergenceRate
	if convergenceProgress < 0 {
		convergenceProgress = 0
	}
	if convergenceProgress > 1.0 {
		convergenceProgress = 1.0
	}
	convergenceText := p.createProgressBar(convergenceProgress, convergenceProgress >= 0.95)
	convergenceText += fmt.Sprintf("\n[white]%.1f%% Stable", convergenceProgress*100)
	p.convergenceBar.SetText(convergenceText)
}

// updateMetricsDisplay refreshes the metrics text view
func (p *Progress) updateMetricsDisplay() {
	if p.currentMetrics == nil {
		p.metricsText.SetText("No data available")
		return
	}

	var builder strings.Builder

	// Basic metrics
	builder.WriteString(fmt.Sprintf("Total Comparisons: [yellow]%d[white]\n",
		p.currentMetrics.TotalComparisons))

	builder.WriteString(fmt.Sprintf("Coverage: [green]%.1f%%[white]\n",
		p.currentMetrics.CoverageComplete*100))

	builder.WriteString(fmt.Sprintf("Convergence Rate: [blue]%.3f[white]\n",
		p.currentMetrics.ConvergenceRate))

	builder.WriteString(fmt.Sprintf("Top %d Stable: [cyan]%d[white]\n",
		p.config.TopNForStability, p.currentMetrics.TopNStable))

	// Confidence scores (show top 5)
	if len(p.currentMetrics.ConfidenceScores) > 0 {
		builder.WriteString("\n[yellow]Top Confidence:[white]\n")
		count := 0
		for proposalID, confidence := range p.currentMetrics.ConfidenceScores {
			if count >= 5 {
				break
			}
			builder.WriteString(fmt.Sprintf("  %s: %.2f\n",
				truncateID(proposalID, 8), confidence))
			count++
		}
	}

	p.metricsText.SetText(builder.String())
}

// updateStatusDisplay refreshes the status text view
func (p *Progress) updateStatusDisplay() {
	if p.currentMetrics == nil {
		p.statusText.SetText("Initializing...")
		return
	}

	var builder strings.Builder

	// Remaining work estimate
	builder.WriteString(fmt.Sprintf("Estimated Remaining: [yellow]%d[white] comparisons\n",
		p.currentMetrics.EstimatedRemaining))

	// Time estimates (if we have comparison history)
	if p.history != nil && len(p.history.Comparisons) > 0 {
		avgDuration := p.calculateAverageComparisonDuration()
		estimatedTime := time.Duration(p.currentMetrics.EstimatedRemaining) * avgDuration

		builder.WriteString(fmt.Sprintf("Estimated Time: [cyan]%s[white]\n",
			formatDuration(estimatedTime)))

		// Session progress
		elapsed := time.Since(p.history.StartTime)
		builder.WriteString(fmt.Sprintf("Session Time: [blue]%s[white]\n",
			formatDuration(elapsed)))
	}

	// Convergence status
	convergenceStatus := p.getConvergenceStatusText()
	builder.WriteString(fmt.Sprintf("\nStatus: %s\n", convergenceStatus))

	// Recommendations
	recommendations := p.getRecommendations()
	if recommendations != "" {
		builder.WriteString(fmt.Sprintf("\n[green]Recommendation:[white]\n%s", recommendations))
	}

	p.statusText.SetText(builder.String())
}

// calculateAverageComparisonDuration calculates average time per comparison
func (p *Progress) calculateAverageComparisonDuration() time.Duration {
	if p.history == nil || len(p.history.Comparisons) == 0 {
		return 30 * time.Second // Default estimate
	}

	totalDuration := time.Duration(0)
	for _, comparison := range p.history.Comparisons {
		totalDuration += comparison.Duration
	}

	return totalDuration / time.Duration(len(p.history.Comparisons))
}

// isConverged determines if the ranking has sufficiently converged
func (p *Progress) isConverged() bool {
	if p.currentMetrics == nil {
		return false
	}

	// Multiple criteria for convergence
	coverageThreshold := p.currentMetrics.CoverageComplete >= 0.8
	convergenceThreshold := p.currentMetrics.ConvergenceRate <= 0.1
	stabilityThreshold := p.currentMetrics.TopNStable >= p.config.TopNForStability

	return coverageThreshold && convergenceThreshold && stabilityThreshold
}

// calculateAverageConfidence calculates overall confidence in rankings
func (p *Progress) calculateAverageConfidence() float64 {
	if p.currentMetrics == nil || len(p.currentMetrics.ConfidenceScores) == 0 {
		return 0.0
	}

	total := 0.0
	count := 0
	for _, confidence := range p.currentMetrics.ConfidenceScores {
		total += confidence
		count++
	}

	return total / float64(count)
}

// getConvergenceStatusText returns a human-readable convergence status
func (p *Progress) getConvergenceStatusText() string {
	if p.currentMetrics == nil {
		return "[gray]Unknown[white]"
	}

	if p.isConverged() {
		return "[green]Converged[white]"
	}

	if p.currentMetrics.CoverageComplete < 0.3 {
		return "[red]Early Stage[white]"
	}

	if p.currentMetrics.CoverageComplete < 0.7 {
		return "[yellow]Progressing[white]"
	}

	return "[blue]Near Convergence[white]"
}

// getRecommendations provides actionable recommendations
func (p *Progress) getRecommendations() string {
	if p.currentMetrics == nil {
		return ""
	}

	if p.isConverged() {
		return "Rankings have converged. Consider exporting results."
	}

	if p.currentMetrics.CoverageComplete < 0.5 {
		return "Continue comparisons to improve coverage."
	}

	if p.currentMetrics.ConvergenceRate > 0.3 {
		return "Rankings still changing significantly. More comparisons recommended."
	}

	if p.currentMetrics.TopNStable < p.config.TopNForStability/2 {
		return "Top rankings not yet stable. Focus on high-priority matchups."
	}

	return "Nearing convergence. A few more comparisons should stabilize rankings."
}

// GetContainer returns the main container for embedding in other views
func (p *Progress) GetContainer() tview.Primitive {
	return p.container
}

// SetVisible controls visibility of the progress component
func (p *Progress) SetVisible(visible bool) {
	if visible {
		p.container.SetTitle("Progress Indicators")
	} else {
		p.container.SetTitle("")
	}
}

// SetConfig updates the optimization configuration
func (p *Progress) SetConfig(config elo.OptimizationConfig) {
	p.config = config
}

// GetMetrics returns the current progress metrics
func (p *Progress) GetMetrics() *elo.ProgressMetrics {
	return p.currentMetrics
}

// truncateID truncates a proposal ID to fit in display
func truncateID(id string, maxLen int) string {
	if len(id) <= maxLen {
		return id
	}
	return id[:maxLen-3] + "..."
}

// createProgressBar creates a visual progress bar using text characters
func (p *Progress) createProgressBar(progress float64, isComplete bool) string {
	const barWidth = 30
	filledWidth := int(progress * barWidth)

	var color string
	if isComplete {
		color = "[green]"
	} else {
		color = "[blue]"
	}

	bar := color + strings.Repeat("█", filledWidth) + "[gray]" + strings.Repeat("░", barWidth-filledWidth) + "[white]"
	return bar
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) - (minutes * 60)
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) - (hours * 60)
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

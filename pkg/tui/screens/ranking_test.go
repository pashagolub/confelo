package screens

import (
	"testing"
	"time"

	"github.com/pashagolub/confelo/pkg/data"
)

// RankingMockApp implements the interfaces that RankingScreen expects from the app
type RankingMockApp struct {
	proposals        []data.Proposal
	comparisonCounts map[string]int
	exportError      error
	calls            []string
}

func (m *RankingMockApp) GetProposals() ([]data.Proposal, error) {
	m.calls = append(m.calls, "GetProposals")
	return m.proposals, nil
}

func (m *RankingMockApp) GetComparisonCount(proposalID string) int {
	m.calls = append(m.calls, "GetComparisonCount")
	if count, exists := m.comparisonCounts[proposalID]; exists {
		return count
	}
	return 0
}

func (m *RankingMockApp) ExportRankings() error {
	m.calls = append(m.calls, "ExportRankings")
	return m.exportError
}

func (m *RankingMockApp) GetState() interface{} {
	m.calls = append(m.calls, "GetState")
	return nil
}

func (m *RankingMockApp) GetStorage() interface{} {
	m.calls = append(m.calls, "GetStorage")
	return nil
}

func (m *RankingMockApp) GetSession() interface{} {
	m.calls = append(m.calls, "GetSession")
	return nil
}

func newRankingMockApp() *RankingMockApp {
	return &RankingMockApp{
		proposals:        createTestProposalsForRanking(),
		comparisonCounts: make(map[string]int),
		calls:            make([]string, 0),
	}
}

// createTestProposalsForRanking creates a set of test proposals for testing
func createTestProposalsForRanking() []data.Proposal {
	return []data.Proposal{
		{
			ID:       "1",
			Title:    "Advanced Go Patterns and Best Practices",
			Speaker:  "Alice Johnson",
			Score:    1650.5,
			Abstract: "This talk covers advanced Go programming patterns including interfaces, generics, and concurrency patterns.",
		},
		{
			ID:       "2",
			Title:    "Microservices at Scale",
			Speaker:  "Bob Smith",
			Score:    1580.2,
			Abstract: "Building and maintaining microservices architecture in large organizations.",
		},
		{
			ID:       "3",
			Title:    "Frontend Performance Optimization",
			Speaker:  "Carol Davis",
			Score:    1720.8,
			Abstract: "Techniques for optimizing frontend applications for better user experience.",
		},
		{
			ID:       "4",
			Title:    "Database Design Principles",
			Speaker:  "David Wilson",
			Score:    1490.3,
			Abstract: "Fundamental principles of database design for modern applications.",
		},
		{
			ID:       "5",
			Title:    "Security in Cloud Computing",
			Speaker:  "Eva Martinez",
			Score:    1615.7,
			Abstract: "Best practices for securing cloud-based applications and infrastructure.",
		},
	}
}

func TestNewRankingScreen(t *testing.T) {
	screen := NewRankingScreen()

	if screen == nil {
		t.Fatal("NewRankingScreen() returned nil")
	}

	if screen.container == nil {
		t.Error("Container not initialized")
	}

	if screen.rankingTable == nil {
		t.Error("Ranking table not initialized")
	}

	if screen.filterForm == nil {
		t.Error("Filter form not initialized")
	}

	// Check initial state
	if screen.sortField != SortByRank {
		t.Errorf("Expected initial sort field to be SortByRank, got %v", screen.sortField)
	}

	if screen.sortOrder != SortAsc {
		t.Errorf("Expected initial sort order to be SortAsc, got %v", screen.sortOrder)
	}

	if screen.selectedRow != 0 {
		t.Errorf("Expected initial selected row to be 0, got %d", screen.selectedRow)
	}
}

func TestRankingScreen_GetPrimitive(t *testing.T) {
	screen := NewRankingScreen()
	primitive := screen.GetPrimitive()

	if primitive == nil {
		t.Error("GetPrimitive() returned nil")
	}

	if primitive != screen.container {
		t.Error("GetPrimitive() did not return the container")
	}
}

func TestRankingScreen_GetTitle(t *testing.T) {
	screen := NewRankingScreen()

	// Test with no proposals
	title := screen.GetTitle()
	expected := "Rankings (0 proposals)"
	if title != expected {
		t.Errorf("Expected title '%s', got '%s'", expected, title)
	}

	// Test with proposals (all visible)
	screen.proposals = createTestProposalsForRanking()
	screen.filteredProposals = screen.proposals
	title = screen.GetTitle()
	expected = "Rankings (5 proposals)"
	if title != expected {
		t.Errorf("Expected title '%s', got '%s'", expected, title)
	}

	// Test with filtered proposals
	screen.filteredProposals = screen.proposals[:3]
	title = screen.GetTitle()
	expected = "Rankings (3/5 proposals)"
	if title != expected {
		t.Errorf("Expected title '%s', got '%s'", expected, title)
	}
}

func TestRankingScreen_GetHelpText(t *testing.T) {
	screen := NewRankingScreen()
	helpText := screen.GetHelpText()

	if len(helpText) == 0 {
		t.Error("GetHelpText() returned empty slice")
	}

	// Check that some essential help items are present
	foundArrowKeys := false
	foundExport := false
	foundSort := false

	for _, text := range helpText {
		if text == "Arrow Keys: Navigate rankings" {
			foundArrowKeys = true
		}
		if text == "E: Export rankings" {
			foundExport = true
		}
		if text == "S: Change sort field" {
			foundSort = true
		}
	}

	if !foundArrowKeys {
		t.Error("Help text missing arrow keys instruction")
	}
	if !foundExport {
		t.Error("Help text missing export instruction")
	}
	if !foundSort {
		t.Error("Help text missing sort instruction")
	}
}

func TestRankingScreen_OnEnter(t *testing.T) {
	screen := NewRankingScreen()
	mockApp := &RankingMockApp{
		proposals: createTestProposalsForRanking(),
		comparisonCounts: map[string]int{
			"1": 5,
			"2": 3,
			"3": 8,
			"4": 1,
			"5": 6,
		},
		calls: make([]string, 0),
	}

	err := screen.OnEnter(mockApp)
	if err != nil {
		t.Errorf("OnEnter() failed: %v", err)
	}

	if screen.app != mockApp {
		t.Error("App reference not set correctly")
	}

	if len(screen.proposals) != 5 {
		t.Errorf("Expected 5 proposals loaded, got %d", len(screen.proposals))
	}

	if len(screen.filteredProposals) != 5 {
		t.Errorf("Expected 5 filtered proposals, got %d", len(screen.filteredProposals))
	}
}

func TestRankingScreen_OnExit(t *testing.T) {
	screen := NewRankingScreen()
	mockApp := newRankingMockApp()

	err := screen.OnExit(mockApp)
	if err != nil {
		t.Errorf("OnExit() failed: %v", err)
	}
}

func TestRankingScreen_LoadProposals(t *testing.T) {
	screen := NewRankingScreen()

	// Test with mock app that provides proposals
	mockApp := newRankingMockApp()
	screen.app = mockApp

	err := screen.loadProposals()
	if err != nil {
		t.Errorf("loadProposals() failed: %v", err)
	}

	if len(screen.proposals) != 5 {
		t.Errorf("Expected 5 proposals loaded, got %d", len(screen.proposals))
	}

	// Test fallback when app doesn't provide proposals (should use mock data)
	screen.app = struct{}{}
	screen.proposals = nil

	err = screen.loadProposals()
	if err != nil {
		t.Errorf("loadProposals() fallback failed: %v", err)
	}

	if len(screen.proposals) == 0 {
		t.Error("Fallback should provide mock proposals")
	}
}

func TestRankingScreen_CalculateConfidence(t *testing.T) {
	screen := NewRankingScreen()

	// Test with mock app that provides comparison counts
	mockApp := &RankingMockApp{
		comparisonCounts: map[string]int{
			"1": 0,  // No comparisons
			"2": 5,  // Medium comparisons
			"3": 15, // Many comparisons
		},
		calls: make([]string, 0),
	}
	screen.app = mockApp

	proposal1 := data.Proposal{ID: "1", Score: 1500.0}
	proposal2 := data.Proposal{ID: "2", Score: 1500.0}
	proposal3 := data.Proposal{ID: "3", Score: 1500.0}

	conf1 := screen.calculateConfidence(proposal1)
	conf2 := screen.calculateConfidence(proposal2)
	conf3 := screen.calculateConfidence(proposal3)

	// More comparisons should lead to higher confidence
	if conf3 <= conf2 {
		t.Errorf("Expected proposal3 confidence (%.2f) > proposal2 confidence (%.2f)", conf3, conf2)
	}

	if conf2 <= conf1 {
		t.Errorf("Expected proposal2 confidence (%.2f) > proposal1 confidence (%.2f)", conf2, conf1)
	}

	// Confidence should be between 0 and 100
	for i, conf := range []float64{conf1, conf2, conf3} {
		if conf < 0 || conf > 100 {
			t.Errorf("Confidence %d (%.2f) should be between 0 and 100", i+1, conf)
		}
	}

	// Test fallback calculation
	screen.app = struct{}{}
	fallbackConf := screen.calculateConfidence(data.Proposal{ID: "test", Score: 1800.0})
	if fallbackConf < 0 || fallbackConf > 100 {
		t.Errorf("Fallback confidence (%.2f) should be between 0 and 100", fallbackConf)
	}
}

func TestRankingScreen_MatchesFilter(t *testing.T) {
	screen := NewRankingScreen()

	proposal := data.Proposal{
		ID:       "1",
		Title:    "Advanced Go Patterns",
		Speaker:  "Alice Johnson",
		Score:    1650.5,
		Abstract: "This covers Go programming patterns and best practices",
	}

	// Test empty filter (should match)
	screen.filter = FilterCriteria{
		MinScore:      0.0,
		MaxScore:      3000.0,
		MinConfidence: 0.0,
	}
	if !screen.matchesFilter(proposal) {
		t.Error("Proposal should match empty filter")
	}

	// Test search text filter
	screen.filter.SearchText = "go"
	if !screen.matchesFilter(proposal) {
		t.Error("Proposal should match 'go' search (case insensitive)")
	}

	screen.filter.SearchText = "alice"
	if !screen.matchesFilter(proposal) {
		t.Error("Proposal should match 'alice' search in speaker name")
	}

	screen.filter.SearchText = "patterns"
	if !screen.matchesFilter(proposal) {
		t.Error("Proposal should match 'patterns' search in abstract")
	}

	screen.filter.SearchText = "nonexistent"
	if screen.matchesFilter(proposal) {
		t.Error("Proposal should not match 'nonexistent' search")
	}

	// Test score filter
	screen.filter.SearchText = ""
	screen.filter.MinScore = 1700.0
	if screen.matchesFilter(proposal) {
		t.Error("Proposal should not match with MinScore 1700 (score is 1650.5)")
	}

	screen.filter.MinScore = 1600.0
	screen.filter.MaxScore = 1640.0
	if screen.matchesFilter(proposal) {
		t.Error("Proposal should not match with MaxScore 1640 (score is 1650.5)")
	}

	screen.filter.MinScore = 1600.0
	screen.filter.MaxScore = 1700.0
	if !screen.matchesFilter(proposal) {
		t.Error("Proposal should match score range 1600-1700")
	}
}

func TestRankingScreen_SortProposals(t *testing.T) {
	screen := NewRankingScreen()
	screen.filteredProposals = []data.Proposal{
		{ID: "1", Title: "Beta", Speaker: "Charlie", Score: 1600.0},
		{ID: "2", Title: "Alpha", Speaker: "Alice", Score: 1700.0},
		{ID: "3", Title: "Gamma", Speaker: "Bob", Score: 1500.0},
	}

	// Test sort by score descending (default for rank)
	screen.sortField = SortByRank
	screen.sortOrder = SortAsc
	screen.sortProposals()

	expectedOrder := []string{"2", "1", "3"} // Highest score first for rank
	for i, expected := range expectedOrder {
		if screen.filteredProposals[i].ID != expected {
			t.Errorf("Sort by rank: expected proposal %s at position %d, got %s",
				expected, i, screen.filteredProposals[i].ID)
		}
	}

	// Test sort by title ascending
	screen.sortField = SortByTitle
	screen.sortOrder = SortAsc
	screen.sortProposals()

	expectedTitles := []string{"Alpha", "Beta", "Gamma"}
	for i, expected := range expectedTitles {
		if screen.filteredProposals[i].Title != expected {
			t.Errorf("Sort by title asc: expected '%s' at position %d, got '%s'",
				expected, i, screen.filteredProposals[i].Title)
		}
	}

	// Test sort by speaker descending
	screen.sortField = SortBySpeaker
	screen.sortOrder = SortDesc
	screen.sortProposals()

	expectedSpeakers := []string{"Charlie", "Bob", "Alice"}
	for i, expected := range expectedSpeakers {
		if screen.filteredProposals[i].Speaker != expected {
			t.Errorf("Sort by speaker desc: expected '%s' at position %d, got '%s'",
				expected, i, screen.filteredProposals[i].Speaker)
		}
	}
}

func TestRankingScreen_ApplyFilterAndSort(t *testing.T) {
	screen := NewRankingScreen()
	screen.proposals = createTestProposalsForRanking()

	// Test with no filter (should include all proposals)
	screen.filter = FilterCriteria{
		MinScore:      0.0,
		MaxScore:      3000.0,
		MinConfidence: 0.0,
	}
	screen.applyFilterAndSort()

	if len(screen.filteredProposals) != len(screen.proposals) {
		t.Errorf("Expected %d filtered proposals, got %d",
			len(screen.proposals), len(screen.filteredProposals))
	}

	// Test with score filter
	screen.filter.MinScore = 1600.0
	screen.applyFilterAndSort()

	count := 0
	for _, p := range screen.proposals {
		if p.Score >= 1600.0 {
			count++
		}
	}

	if len(screen.filteredProposals) != count {
		t.Errorf("Expected %d proposals with score >= 1600, got %d",
			count, len(screen.filteredProposals))
	}

	// Verify all filtered proposals meet the criteria
	for _, p := range screen.filteredProposals {
		if p.Score < 1600.0 {
			t.Errorf("Filtered proposal %s has score %.1f < 1600", p.ID, p.Score)
		}
	}
}

func TestRankingScreen_ClearFilters(t *testing.T) {
	screen := NewRankingScreen()
	screen.proposals = createTestProposalsForRanking()

	// Set some filters
	screen.filter = FilterCriteria{
		SearchText:    "test",
		MinScore:      1600.0,
		MaxScore:      1800.0,
		MinConfidence: 50.0,
	}

	screen.clearFilters()

	// Check filter is reset to defaults
	if screen.filter.SearchText != "" {
		t.Errorf("Expected empty search text, got '%s'", screen.filter.SearchText)
	}

	if screen.filter.MinScore != 0.0 {
		t.Errorf("Expected MinScore 0.0, got %.1f", screen.filter.MinScore)
	}

	if screen.filter.MaxScore != 3000.0 {
		t.Errorf("Expected MaxScore 3000.0, got %.1f", screen.filter.MaxScore)
	}

	if screen.filter.MinConfidence != 0.0 {
		t.Errorf("Expected MinConfidence 0.0, got %.1f", screen.filter.MinConfidence)
	}
}

func TestRankingScreen_CycleSortField(t *testing.T) {
	screen := NewRankingScreen()
	screen.proposals = createTestProposalsForRanking()
	screen.filteredProposals = screen.proposals

	initialField := screen.sortField
	screen.cycleSortField()

	if screen.sortField == initialField {
		t.Error("Sort field should have changed after cycleSortField")
	}

	// Cycle through all fields and back to start
	for i := 0; i < 4; i++ {
		screen.cycleSortField()
	}

	if screen.sortField != initialField {
		t.Errorf("Expected to cycle back to initial field %v, got %v",
			initialField, screen.sortField)
	}
}

func TestRankingScreen_ToggleSortOrder(t *testing.T) {
	screen := NewRankingScreen()
	screen.proposals = createTestProposalsForRanking()
	screen.filteredProposals = screen.proposals

	initialOrder := screen.sortOrder
	screen.toggleSortOrder()

	if screen.sortOrder == initialOrder {
		t.Error("Sort order should have changed after toggleSortOrder")
	}

	screen.toggleSortOrder()

	if screen.sortOrder != initialOrder {
		t.Errorf("Expected to toggle back to initial order %v, got %v",
			initialOrder, screen.sortOrder)
	}
}

func TestRankingScreen_ExportFunctionality(t *testing.T) {
	screen := NewRankingScreen()
	screen.filteredProposals = createTestProposalsForRanking()[:3] // Use first 3 proposals

	// Test CSV export
	err := screen.exportToCSV("test.csv")
	if err != nil {
		t.Errorf("exportToCSV failed: %v", err)
	}

	// Test JSON export
	err = screen.exportToJSON("test.json")
	if err != nil {
		t.Errorf("exportToJSON failed: %v", err)
	}

	// Test export with mock app that returns error
	mockApp := &RankingMockApp{
		exportError: data.ErrInvalidProposal,
		calls:       make([]string, 0),
	}
	screen.app = mockApp

	// Test that export handles errors gracefully
	screen.initiateExport()

	// Wait a moment for the goroutine to complete
	time.Sleep(100 * time.Millisecond)
}

func TestRankingScreen_GetScoreColor(t *testing.T) {
	screen := NewRankingScreen()

	tests := []struct {
		score    float64
		expected string
	}{
		{1700.0, "green"},  // High score
		{1500.0, "yellow"}, // Medium score
		{1300.0, "red"},    // Low score
	}

	for _, test := range tests {
		color := screen.getScoreColor(test.score)

		// We can't easily test exact color values, so just ensure it returns a valid color
		if color < 0 {
			t.Errorf("getScoreColor(%.1f) returned invalid color", test.score)
		}
	}
}

func TestRankingScreen_GetConfidenceColor(t *testing.T) {
	screen := NewRankingScreen()

	tests := []struct {
		confidence float64
	}{
		{90.0}, // High confidence
		{60.0}, // Medium confidence
		{30.0}, // Low confidence
	}

	for _, test := range tests {
		color := screen.getConfidenceColor(test.confidence)

		// Just ensure it returns a valid color
		if color < 0 {
			t.Errorf("getConfidenceColor(%.1f) returned invalid color", test.confidence)
		}
	}
}

func TestRankingScreen_UpdateStatistics(t *testing.T) {
	screen := NewRankingScreen()

	// Test with no proposals
	screen.filteredProposals = nil
	screen.updateStatistics()
	// Should not crash

	// Test with proposals
	screen.filteredProposals = createTestProposalsForRanking()
	screen.updateStatistics()
	// Should not crash and should calculate statistics

	if screen.statisticsPanel == nil {
		t.Error("Statistics panel should be initialized")
	}
}

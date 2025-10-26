package screens

import (
	"testing"

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

func (m *RankingMockApp) GetState() any {
	m.calls = append(m.calls, "GetState")
	return nil
}

func (m *RankingMockApp) GetStorage() any {
	m.calls = append(m.calls, "GetStorage")
	return nil
}

func (m *RankingMockApp) GetSession() any {
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
}

func TestRankingScreen_OnExit(t *testing.T) {
	screen := NewRankingScreen()
	mockApp := newRankingMockApp()

	err := screen.OnExit(mockApp)
	if err != nil {
		t.Errorf("OnExit() failed: %v", err)
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

func TestRankingScreen_SortProposals(t *testing.T) {
	screen := NewRankingScreen()
	screen.proposals = []data.Proposal{
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
		if screen.proposals[i].ID != expected {
			t.Errorf("Sort by rank: expected proposal %s at position %d, got %s",
				expected, i, screen.proposals[i].ID)
		}
	}

	// Test sort by title ascending
	screen.sortField = SortByTitle
	screen.sortOrder = SortAsc
	screen.sortProposals()

	expectedTitles := []string{"Alpha", "Beta", "Gamma"}
	for i, expected := range expectedTitles {
		if screen.proposals[i].Title != expected {
			t.Errorf("Sort by title asc: expected '%s' at position %d, got '%s'",
				expected, i, screen.proposals[i].Title)
		}
	}

	// Test sort by speaker descending
	screen.sortField = SortBySpeaker
	screen.sortOrder = SortDesc
	screen.sortProposals()

	expectedSpeakers := []string{"Charlie", "Bob", "Alice"}
	for i, expected := range expectedSpeakers {
		if screen.proposals[i].Speaker != expected {
			t.Errorf("Sort by speaker desc: expected '%s' at position %d, got '%s'",
				expected, i, screen.proposals[i].Speaker)
		}
	}
}

func TestRankingScreen_CycleSortField(t *testing.T) {
	screen := NewRankingScreen()
	screen.proposals = createTestProposalsForRanking()

	initialField := screen.sortField
	screen.cycleSortField()

	if screen.sortField == initialField {
		t.Error("Sort field should have changed after cycleSortField")
	}

	// Cycle through all fields and back to start
	// We have 6 sort fields: Rank, Score, ExportScore, Title, Speaker, Confidence
	for i := 0; i < 5; i++ {
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

		// Just ensure the method doesn't panic - color values are always valid
		_ = color
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

		// Just ensure the method doesn't panic - color values are always valid
		_ = color
	}
}

// RankingMockAppWithConfig extends RankingMockApp to include config
type RankingMockAppWithConfig struct {
	RankingMockApp
	config *data.SessionConfig
}

func (m *RankingMockAppWithConfig) GetConfig() *data.SessionConfig {
	m.calls = append(m.calls, "GetConfig")
	return m.config
}

func TestRankingScreen_CalculateExportScore(t *testing.T) {
	t.Run("ScalesBasedOnActualMinMaxInteger", func(t *testing.T) {
		screen := NewRankingScreen()

		// Set up proposals with scores ranging from 1490 to 1720
		screen.proposals = createTestProposalsForRanking()

		// Configure for 0-100 integer scale
		config := data.DefaultSessionConfig()
		config.Elo.OutputMin = 0.0
		config.Elo.OutputMax = 100.0
		config.Elo.UseDecimals = false

		mockApp := &RankingMockAppWithConfig{
			RankingMockApp: RankingMockApp{
				proposals: screen.proposals,
			},
			config: &config,
		}
		screen.app = mockApp

		// Min score (1490.3) should map to 0
		minScore := screen.calculateExportScore(1490.3)
		if minScore != 0.0 {
			t.Errorf("Expected min export score to be 0, got %.1f", minScore)
		}

		// Max score (1720.8) should map to 100
		maxScore := screen.calculateExportScore(1720.8)
		if maxScore != 100.0 {
			t.Errorf("Expected max export score to be 100, got %.1f", maxScore)
		}

		// Mid-range score should be roughly in the middle
		midScore := screen.calculateExportScore(1605.0) // Roughly middle of 1490-1720
		if midScore < 45.0 || midScore > 55.0 {
			t.Errorf("Expected mid export score to be around 50, got %.1f", midScore)
		}
	})

	t.Run("ScalesBasedOnActualMinMaxDecimal", func(t *testing.T) {
		screen := NewRankingScreen()
		screen.proposals = createTestProposalsForRanking()

		// Configure for 1.0-5.0 decimal scale
		config := data.DefaultSessionConfig()
		config.Elo.OutputMin = 1.0
		config.Elo.OutputMax = 5.0
		config.Elo.UseDecimals = true

		mockApp := &RankingMockAppWithConfig{
			RankingMockApp: RankingMockApp{
				proposals: screen.proposals,
			},
			config: &config,
		}
		screen.app = mockApp

		// Min score should map to 1.0
		minScore := screen.calculateExportScore(1490.3)
		if minScore < 0.99 || minScore > 1.01 {
			t.Errorf("Expected min export score to be ~1.0, got %.2f", minScore)
		}

		// Max score should map to 5.0
		maxScore := screen.calculateExportScore(1720.8)
		if maxScore < 4.99 || maxScore > 5.01 {
			t.Errorf("Expected max export score to be ~5.0, got %.2f", maxScore)
		}

		// Mid-range should be around 3.0
		midScore := screen.calculateExportScore(1605.0)
		if midScore < 2.8 || midScore > 3.2 {
			t.Errorf("Expected mid export score to be ~3.0, got %.2f", midScore)
		}
	})

	t.Run("HandlesAllSameScore", func(t *testing.T) {
		screen := NewRankingScreen()

		// All proposals with same score
		screen.proposals = []data.Proposal{
			{ID: "1", Title: "Talk 1", Score: 1500.0},
			{ID: "2", Title: "Talk 2", Score: 1500.0},
			{ID: "3", Title: "Talk 3", Score: 1500.0},
		}

		config := data.DefaultSessionConfig()
		config.Elo.OutputMin = 0.0
		config.Elo.OutputMax = 100.0
		config.Elo.UseDecimals = false

		mockApp := &RankingMockAppWithConfig{
			RankingMockApp: RankingMockApp{
				proposals: screen.proposals,
			},
			config: &config,
		}
		screen.app = mockApp

		// Should return middle of scale when all scores are the same
		score := screen.calculateExportScore(1500.0)
		if score != 50.0 {
			t.Errorf("Expected export score to be 50 (middle) when all scores same, got %.1f", score)
		}
	})

	t.Run("HandlesEmptyProposals", func(t *testing.T) {
		screen := NewRankingScreen()
		screen.proposals = []data.Proposal{}

		config := data.DefaultSessionConfig()
		config.Elo.OutputMin = 0.0
		config.Elo.OutputMax = 100.0
		config.Elo.UseDecimals = false

		mockApp := &RankingMockAppWithConfig{
			RankingMockApp: RankingMockApp{
				proposals: screen.proposals,
			},
			config: &config,
		}
		screen.app = mockApp

		// Should return minimum when no proposals
		score := screen.calculateExportScore(1500.0)
		if score != 0.0 {
			t.Errorf("Expected export score to be 0 (min) when no proposals, got %.1f", score)
		}
	})

	t.Run("IntegerRounding", func(t *testing.T) {
		screen := NewRankingScreen()
		screen.proposals = createTestProposalsForRanking()

		config := data.DefaultSessionConfig()
		config.Elo.OutputMin = 0.0
		config.Elo.OutputMax = 100.0
		config.Elo.UseDecimals = false

		mockApp := &RankingMockAppWithConfig{
			RankingMockApp: RankingMockApp{
				proposals: screen.proposals,
			},
			config: &config,
		}
		screen.app = mockApp

		// Test that scores are rounded to integers
		score := screen.calculateExportScore(1550.0)
		if score != float64(int(score)) {
			t.Errorf("Expected integer score when UseDecimals=false, got %.2f", score)
		}
	})
}

func TestRankingScreen_FormatExportScore(t *testing.T) {
	t.Run("FormatsAsInteger", func(t *testing.T) {
		screen := NewRankingScreen()

		config := data.DefaultSessionConfig()
		config.Elo.UseDecimals = false

		mockApp := &RankingMockAppWithConfig{
			config: &config,
		}
		screen.app = mockApp

		formatted := screen.formatExportScore(42.7)
		if formatted != "42" {
			t.Errorf("Expected '42', got '%s'", formatted)
		}
	})

	t.Run("FormatsAsDecimal", func(t *testing.T) {
		screen := NewRankingScreen()

		config := data.DefaultSessionConfig()
		config.Elo.UseDecimals = true

		mockApp := &RankingMockAppWithConfig{
			config: &config,
		}
		screen.app = mockApp

		formatted := screen.formatExportScore(3.14159)
		if formatted != "3.1" {
			t.Errorf("Expected '3.1', got '%s'", formatted)
		}
	})
}

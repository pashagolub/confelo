package data

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test data constants
const (
	testCSVWithHeader = `id,title,abstract,speaker,score,conflict,extra_field
TALK-001,"Go Concurrency Patterns","Learn about channels and goroutines","John Doe",85.5,reviewer1;reviewer2,"some extra data"
TALK-002,"Web Development with Go","Building REST APIs","Jane Smith",92.0,reviewer3,"more data"
TALK-003,"Testing in Go","Best practices for testing","Bob Wilson",88.0,,"additional info"`

	testCSVNoHeader = `TALK-004,Docker Containers,Containerization basics,Alice Cooper,90.0,,extra
TALK-005,Kubernetes Deep Dive,Container orchestration,Mike Johnson,87.5,reviewer1,info`
)

func TestNewProposal(t *testing.T) {
	tests := []struct {
		name          string
		id            string
		title         string
		config        ValidationConfig
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid proposal",
			id:          "TALK-001",
			title:       "Test Title",
			config:      ValidationConfig{},
			expectError: false,
		},
		{
			name:          "empty id",
			id:            "",
			title:         "Test Title",
			config:        ValidationConfig{},
			expectError:   true,
			errorContains: "ID cannot be empty",
		},
		{
			name:          "empty title when required",
			id:            "TALK-001",
			title:         "",
			config:        ValidationConfig{RequireTitle: true, MaxTitleLength: 100},
			expectError:   true,
			errorContains: "title is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proposal, err := NewProposal(tt.id, tt.title, tt.config)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, proposal)
				assert.Equal(t, tt.id, proposal.ID)
				assert.Equal(t, tt.title, proposal.Title)
				assert.NotNil(t, proposal.Metadata)
				assert.NotNil(t, proposal.ConflictTags)
				assert.False(t, proposal.CreatedAt.IsZero())
				assert.False(t, proposal.UpdatedAt.IsZero())
			}
		})
	}
}

func TestProposal_Validate(t *testing.T) {
	tests := []struct {
		name     string
		proposal *Proposal
		config   ValidationConfig
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid proposal",
			proposal: &Proposal{
				ID:    "TALK-001",
				Title: "Test Title",
				Score: 85.0,
			},
			config:  ValidationConfig{RequireTitle: true, MaxTitleLength: 100, MaxScore: 100},
			wantErr: false,
		},
		{
			name: "missing ID",
			proposal: &Proposal{
				ID:    "",
				Title: "Test Title",
			},
			config:  ValidationConfig{RequireTitle: true, MaxTitleLength: 100},
			wantErr: true,
			errMsg:  "ID is required",
		},
		{
			name: "missing title when required",
			proposal: &Proposal{
				ID:    "TALK-001",
				Title: "",
			},
			config:  ValidationConfig{RequireTitle: true, MaxTitleLength: 100},
			wantErr: true,
			errMsg:  "title is required",
		},
		{
			name: "score below minimum",
			proposal: &Proposal{
				ID:    "TALK-001",
				Title: "Test Title",
				Score: 50.0,
			},
			config: ValidationConfig{
				RequireTitle:   true,
				MaxTitleLength: 100,
				MinScore:       60.0,
				MaxScore:       100.0,
			},
			wantErr: true,
			errMsg:  "outside valid range",
		},
		{
			name: "score above maximum",
			proposal: &Proposal{
				ID:    "TALK-001",
				Title: "Test Title",
				Score: 110.0,
			},
			config: ValidationConfig{
				RequireTitle:   true,
				MaxTitleLength: 100,
				MaxScore:       100.0,
			},
			wantErr: true,
			errMsg:  "outside valid range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.proposal.Validate(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProposal_Metadata(t *testing.T) {
	config := ValidationConfig{}
	proposal, err := NewProposal("TALK-001", "Test Title", config)
	require.NoError(t, err)
	require.NotNil(t, proposal)

	// Test setting metadata
	proposal.SetMetadata("category", "technical")
	proposal.SetMetadata("difficulty", "intermediate")

	// Test getting metadata
	value, exists := proposal.GetMetadata("category")
	assert.True(t, exists)
	assert.Equal(t, "technical", value)

	value, exists = proposal.GetMetadata("difficulty")
	assert.True(t, exists)
	assert.Equal(t, "intermediate", value)

	_, exists = proposal.GetMetadata("nonexistent")
	assert.False(t, exists)
}

func TestProposal_ConflictTags(t *testing.T) {
	config := ValidationConfig{}
	proposal, err := NewProposal("TALK-001", "Test Title", config)
	require.NoError(t, err)
	require.NotNil(t, proposal)

	// Test adding conflict tags
	proposal.AddConflictTag("reviewer1")
	proposal.AddConflictTag("reviewer2")

	// Test duplicate tag (should not be added twice)
	proposal.AddConflictTag("reviewer1")

	assert.Len(t, proposal.ConflictTags, 2)
	assert.Contains(t, proposal.ConflictTags, "reviewer1")
	assert.Contains(t, proposal.ConflictTags, "reviewer2")

	// Test checking for conflict
	assert.True(t, proposal.HasConflictTag("reviewer1"))
	assert.True(t, proposal.HasConflictTag("reviewer2"))
	assert.False(t, proposal.HasConflictTag("reviewer3"))

	// Test removing conflict tag
	proposal.RemoveConflictTag("reviewer1")
	assert.Len(t, proposal.ConflictTags, 1)
	assert.False(t, proposal.HasConflictTag("reviewer1"))
	assert.True(t, proposal.HasConflictTag("reviewer2"))
}

func TestProposal_UpdateScore(t *testing.T) {
	config := ValidationConfig{MaxScore: 100}
	proposal, err := NewProposal("TALK-001", "Test Title", config)
	require.NoError(t, err)
	require.NotNil(t, proposal)

	// Test initial score (uses DefaultScore from config)
	assert.Equal(t, 0.0, proposal.Score)
	assert.Nil(t, proposal.OriginalScore)

	// Test setting score
	createdAt := proposal.CreatedAt
	// Sleep briefly to ensure time difference
	time.Sleep(time.Millisecond)
	proposal.UpdateScore(85.5)
	assert.Equal(t, 85.5, proposal.Score)
	assert.True(t, proposal.UpdatedAt.After(createdAt))

	// OriginalScore is only set during CSV parsing, not by UpdateScore
	assert.Nil(t, proposal.OriginalScore)

	// Change score again
	firstUpdate := proposal.UpdatedAt
	time.Sleep(time.Millisecond)
	proposal.UpdateScore(90.0)
	assert.Equal(t, 90.0, proposal.Score)
	assert.True(t, proposal.UpdatedAt.After(firstUpdate))
	assert.Nil(t, proposal.OriginalScore) // Still nil
}

func TestParseCSVFromReader_WithHeaders(t *testing.T) {
	reader := strings.NewReader(testCSVWithHeader)
	csvConfig := CSVConfig{
		HasHeader:      true,
		IDColumn:       "id",
		TitleColumn:    "title",
		AbstractColumn: "abstract",
		SpeakerColumn:  "speaker",
		ScoreColumn:    "score",
		ConflictColumn: "conflict",
	}
	validationConfig := ValidationConfig{
		RequireTitle:   true,
		MaxTitleLength: 200,
		MinScore:       0.0,
		MaxScore:       100.0,
		DefaultScore:   75.0,
	}

	result, err := ParseCSVFromReader(reader, csvConfig, validationConfig, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check metadata
	assert.Len(t, result.Metadata.Headers, 7)
	assert.Equal(t, "id", result.Metadata.Headers[0])
	assert.Equal(t, "extra_field", result.Metadata.Headers[6])

	// Check proposals
	assert.Len(t, result.Proposals, 3)
	assert.Equal(t, 3, result.SuccessfulRows)
	assert.Equal(t, 4, result.TotalRows) // Including header

	// Test first proposal
	p1 := result.Proposals[0]
	assert.Equal(t, "TALK-001", p1.ID)
	assert.Equal(t, "Go Concurrency Patterns", p1.Title)
	assert.Equal(t, "Learn about channels and goroutines", p1.Abstract)
	assert.Equal(t, "John Doe", p1.Speaker)
	assert.Equal(t, 85.5, p1.Score)
	assert.NotNil(t, p1.OriginalScore)
	assert.Equal(t, 85.5, *p1.OriginalScore)

	// Test conflict tags
	assert.Len(t, p1.ConflictTags, 2)
	assert.Contains(t, p1.ConflictTags, "reviewer1")
	assert.Contains(t, p1.ConflictTags, "reviewer2")

	// Test metadata preservation
	value, exists := p1.GetMetadata("extra_field")
	assert.True(t, exists)
	assert.Equal(t, "some extra data", value)
}

func TestParseCSVFromReader_NoHeaders(t *testing.T) {
	reader := strings.NewReader(testCSVNoHeader)
	csvConfig := CSVConfig{
		HasHeader:      false,
		IDColumn:       "0", // Column index
		TitleColumn:    "1",
		AbstractColumn: "2",
		SpeakerColumn:  "3",
		ScoreColumn:    "4",
		ConflictColumn: "5",
	}
	validationConfig := ValidationConfig{
		RequireTitle:   true,
		MaxTitleLength: 200,
		MaxScore:       100,
		DefaultScore:   75.0,
	}

	result, err := ParseCSVFromReader(reader, csvConfig, validationConfig, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check proposals
	assert.Len(t, result.Proposals, 2)
	assert.Equal(t, 2, result.SuccessfulRows)
	assert.Equal(t, 2, result.TotalRows)

	// Test first proposal
	p1 := result.Proposals[0]
	assert.Equal(t, "TALK-004", p1.ID)
	assert.Equal(t, "Docker Containers", p1.Title)
	assert.Equal(t, "Containerization basics", p1.Abstract)
	assert.Equal(t, "Alice Cooper", p1.Speaker)
	assert.Equal(t, 90.0, p1.Score)
}

func TestProposalCollection_Basic(t *testing.T) {
	config := ValidationConfig{}
	collection := NewProposalCollection(config)

	// Create test proposals
	proposal1, _ := NewProposal("TALK-001", "Title 1", config)
	proposal1.AddConflictTag("reviewer1")
	proposal1.AddConflictTag("reviewer2")

	proposal2, _ := NewProposal("TALK-002", "Title 2", config)
	proposal2.AddConflictTag("reviewer2")

	proposal3, _ := NewProposal("TALK-003", "Title 3", config)

	// Test adding proposals
	err := collection.AddProposal(*proposal1)
	assert.NoError(t, err)
	err = collection.AddProposal(*proposal2)
	assert.NoError(t, err)
	err = collection.AddProposal(*proposal3)
	assert.NoError(t, err)

	// Test count
	assert.Equal(t, 3, collection.Count())

	// Test getting by ID
	found, exists := collection.GetProposalByID("TALK-001")
	assert.True(t, exists)
	require.NotNil(t, found)
	assert.Equal(t, "TALK-001", found.ID)
	assert.Equal(t, "Title 1", found.Title)

	// Test not found
	_, exists = collection.GetProposalByID("TALK-999")
	assert.False(t, exists)

	// Test getting IDs
	ids := collection.IDs()
	assert.Len(t, ids, 3)
	assert.Contains(t, ids, "TALK-001")
	assert.Contains(t, ids, "TALK-002")
	assert.Contains(t, ids, "TALK-003")

	// Test filtering by conflict tag
	filtered := collection.FilterByConflictTag("reviewer1")
	assert.Len(t, filtered, 1)
	assert.Equal(t, "TALK-001", filtered[0].ID)

	// Test excluding by conflict tag
	excluded := collection.ExcludeByConflictTag("reviewer1")
	assert.Len(t, excluded, 2)
}

func TestProposalCollection_Update(t *testing.T) {
	config := ValidationConfig{MaxScore: 100, MaxTitleLength: 200}
	collection := NewProposalCollection(config)

	// Create and add initial proposal
	proposal1, _ := NewProposal("TALK-001", "Title 1", config)
	proposal1.UpdateScore(85.0)
	collection.AddProposal(*proposal1)

	// Update proposal
	updated, _ := NewProposal("TALK-001", "Updated Title", config)
	updated.UpdateScore(95.0)

	err := collection.UpdateProposal(*updated)
	assert.NoError(t, err)

	// Verify update
	found, exists := collection.GetProposalByID("TALK-001")
	assert.True(t, exists)
	require.NotNil(t, found)
	assert.Equal(t, "Updated Title", found.Title)
	assert.Equal(t, 95.0, found.Score)

	// Try to update non-existent proposal
	nonExistent, _ := NewProposal("TALK-999", "Non-existent", config)
	err = collection.UpdateProposal(*nonExistent)
	assert.Error(t, err)
}

func TestCSVParseError(t *testing.T) {
	parseError := CSVParseError{
		RowNumber: 5,
		Field:     "score",
		Value:     "invalid",
		Message:   "invalid number format",
	}

	// Test the error message formatting
	assert.Equal(t, 5, parseError.RowNumber)
	assert.Equal(t, "score", parseError.Field)
	assert.Equal(t, "invalid", parseError.Value)
	assert.Equal(t, "invalid number format", parseError.Message)

	// Test Error() method
	expected := "row 5, field 'score' (value: 'invalid'): invalid number format"
	assert.Equal(t, expected, parseError.Error())
}

func TestValidationConfig_Defaults(t *testing.T) {
	config := ValidationConfig{}

	// Test default values
	assert.False(t, config.RequireTitle)
	assert.Equal(t, 0.0, config.MinScore)
	assert.Equal(t, 0.0, config.MaxScore)
	assert.Equal(t, 0.0, config.DefaultScore)
}

// Benchmark tests
func BenchmarkProposal_UpdateScore(b *testing.B) {
	config := ValidationConfig{}
	proposal, _ := NewProposal("TALK-001", "Test Title", config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proposal.UpdateScore(float64(i % 100))
	}
}

func BenchmarkProposal_AddConflictTag(b *testing.B) {
	config := ValidationConfig{}
	proposal, _ := NewProposal("TALK-001", "Test Title", config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proposal.AddConflictTag(fmt.Sprintf("reviewer%d", i%10))
	}
}

func BenchmarkParseCSVFromReader(b *testing.B) {
	csvConfig := CSVConfig{
		HasHeader:      true,
		IDColumn:       "id",
		TitleColumn:    "title",
		AbstractColumn: "abstract",
		SpeakerColumn:  "speaker",
		ScoreColumn:    "score",
		ConflictColumn: "conflict",
	}
	validationConfig := ValidationConfig{
		RequireTitle: true,
		DefaultScore: 75.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(testCSVWithHeader)
		_, err := ParseCSVFromReader(reader, csvConfig, validationConfig, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

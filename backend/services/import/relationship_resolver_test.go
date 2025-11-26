package importpkg

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/stretchr/testify/assert"
)

func TestRelationshipResolver_ResolveGroup_ExactMatch(t *testing.T) {
	resolver := &RelationshipResolver{
		groupCache: map[string]*education.Group{
			"gruppe 1a": {Model: base.Model{ID: 1}, Name: "Gruppe 1A"},
			"gruppe 2b": {Model: base.Model{ID: 2}, Name: "Gruppe 2B"},
			"gruppe 3c": {Model: base.Model{ID: 3}, Name: "Gruppe 3C"},
		},
	}

	tests := []struct {
		name     string
		input    string
		wantID   *int64
		wantErrs bool
	}{
		{
			name:     "exact match lowercase",
			input:    "Gruppe 1A",
			wantID:   int64Ptr(1),
			wantErrs: false,
		},
		{
			name:     "exact match case insensitive",
			input:    "gruppe 1a",
			wantID:   int64Ptr(1),
			wantErrs: false,
		},
		{
			name:     "exact match with spaces",
			input:    "  Gruppe 1A  ",
			wantID:   int64Ptr(1),
			wantErrs: false,
		},
		{
			name:     "empty input",
			input:    "",
			wantID:   nil,
			wantErrs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, errors := resolver.ResolveGroup(context.Background(), tt.input)

			if tt.wantID != nil {
				assert.NotNil(t, id, "Expected ID to be non-nil")
				assert.Equal(t, *tt.wantID, *id, "ID mismatch")
				assert.Empty(t, errors, "Expected no errors")
			} else if tt.wantErrs {
				assert.Nil(t, id, "Expected ID to be nil")
				assert.NotEmpty(t, errors, "Expected errors")
			} else {
				assert.Nil(t, id, "Expected ID to be nil")
				assert.Empty(t, errors, "Expected no errors")
			}
		})
	}
}

func TestRelationshipResolver_ResolveGroup_FuzzyMatch(t *testing.T) {
	resolver := &RelationshipResolver{
		groupCache: map[string]*education.Group{
			"gruppe 1a":  {Model: base.Model{ID: 1}, Name: "Gruppe 1A"},
			"gruppe 2b":  {Model: base.Model{ID: 2}, Name: "Gruppe 2B"},
			"gruppe 10a": {Model: base.Model{ID: 10}, Name: "Gruppe 10A"},
		},
	}

	tests := []struct {
		name                  string
		input                 string
		wantSuggestions       bool
		expectedInSuggestions []string
	}{
		{
			name:                  "close match - missing number",
			input:                 "Gruppe A",
			wantSuggestions:       true,
			expectedInSuggestions: []string{"Gruppe 1A"}, // Should suggest 1A
		},
		{
			name:                  "typo - single character difference",
			input:                 "Gruppe 1B",
			wantSuggestions:       true,
			expectedInSuggestions: []string{"Gruppe 1A", "Gruppe 2B"}, // Both close
		},
		{
			name:                  "typo - transposed characters",
			input:                 "Gruppe 01A",
			wantSuggestions:       true,
			expectedInSuggestions: []string{"Gruppe 10A", "Gruppe 1A"}, // Both close
		},
		{
			name:                  "no match - too different",
			input:                 "Abteilung ABC",
			wantSuggestions:       false,
			expectedInSuggestions: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, errors := resolver.ResolveGroup(context.Background(), tt.input)

			assert.Nil(t, id, "Expected ID to be nil for fuzzy/no match")
			assert.NotEmpty(t, errors, "Expected errors")

			if tt.wantSuggestions {
				assert.NotEmpty(t, errors[0].Suggestions, "Expected suggestions")
				assert.Equal(t, importModels.ErrorSeverityWarning, errors[0].Severity) // Warning, not error - allows import without group
				assert.Equal(t, "group_not_found_with_suggestions", errors[0].Code)
				assert.NotNil(t, errors[0].AutoFix, "Expected AutoFix")

				// Check if expected suggestions are present
				for _, expected := range tt.expectedInSuggestions {
					assert.Contains(t, errors[0].Suggestions, expected, "Missing expected suggestion")
				}
			} else {
				// No suggestions case
				assert.Empty(t, errors[0].Suggestions, "Expected no suggestions")
				assert.Equal(t, importModels.ErrorSeverityWarning, errors[0].Severity) // Warning, not error
				assert.Equal(t, "group_not_found", errors[0].Code)
			}
		})
	}
}

func TestRelationshipResolver_ResolveRoom_ExactMatch(t *testing.T) {
	resolver := &RelationshipResolver{
		roomCache: map[string]*facilities.Room{
			"raum 101": {Model: base.Model{ID: 1}, Name: "Raum 101"},
			"raum 202": {Model: base.Model{ID: 2}, Name: "Raum 202"},
		},
	}

	tests := []struct {
		name   string
		input  string
		wantID *int64
	}{
		{
			name:   "exact match",
			input:  "Raum 101",
			wantID: int64Ptr(1),
		},
		{
			name:   "case insensitive",
			input:  "raum 101",
			wantID: int64Ptr(1),
		},
		{
			name:   "with spaces",
			input:  "  Raum 101  ",
			wantID: int64Ptr(1),
		},
		{
			name:   "empty",
			input:  "",
			wantID: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, errors := resolver.ResolveRoom(context.Background(), tt.input)

			if tt.wantID != nil {
				assert.NotNil(t, id)
				assert.Equal(t, *tt.wantID, *id)
				assert.Empty(t, errors)
			} else {
				assert.Nil(t, id)
				assert.Empty(t, errors)
			}
		})
	}
}

func TestRelationshipResolver_ResolveRoom_FuzzyMatch(t *testing.T) {
	resolver := &RelationshipResolver{
		roomCache: map[string]*facilities.Room{
			"raum 101": {Model: base.Model{ID: 1}, Name: "Raum 101"},
			"raum 102": {Model: base.Model{ID: 2}, Name: "Raum 102"},
			"raum 201": {Model: base.Model{ID: 3}, Name: "Raum 201"},
		},
	}

	tests := []struct {
		name            string
		input           string
		wantSuggestions bool
	}{
		{
			name:            "typo - single char difference",
			input:           "Raum 10",
			wantSuggestions: true,
		},
		{
			name:            "typo - transposed digit",
			input:           "Raum 011",
			wantSuggestions: true,
		},
		{
			name:            "no match",
			input:           "Zimmer ABC",
			wantSuggestions: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, errors := resolver.ResolveRoom(context.Background(), tt.input)

			assert.Nil(t, id)
			assert.NotEmpty(t, errors)

			if tt.wantSuggestions {
				assert.NotEmpty(t, errors[0].Suggestions)
				assert.Equal(t, "room_not_found_with_suggestions", errors[0].Code)
				assert.NotNil(t, errors[0].AutoFix)
			} else {
				assert.Empty(t, errors[0].Suggestions)
				assert.Equal(t, "room_not_found", errors[0].Code)
			}
		})
	}
}

func TestRelationshipResolver_FindSimilarGroups(t *testing.T) {
	resolver := &RelationshipResolver{
		groupCache: map[string]*education.Group{
			"gruppe 1a":  {Model: base.Model{ID: 1}, Name: "Gruppe 1A"},
			"gruppe 1b":  {Model: base.Model{ID: 2}, Name: "Gruppe 1B"},
			"gruppe 2a":  {Model: base.Model{ID: 3}, Name: "Gruppe 2A"},
			"gruppe 10a": {Model: base.Model{ID: 4}, Name: "Gruppe 10A"},
		},
	}

	tests := []struct {
		name        string
		input       string
		maxDistance int
		minMatches  int
		maxMatches  int
		firstMatch  string // Expected first suggestion (closest)
	}{
		{
			name:        "distance 1 - should find 1A and 1B",
			input:       "Gruppe 1C",
			maxDistance: 1,
			minMatches:  2,
			maxMatches:  3,
			firstMatch:  "Gruppe 1A", // or 1B, both distance 1
		},
		{
			name:        "distance 2 - should find multiple",
			input:       "Gruppe A",
			maxDistance: 3,
			minMatches:  1,
			maxMatches:  3,
			firstMatch:  "Gruppe 1A",
		},
		{
			name:        "distance 0 - exact match not found by findSimilar",
			input:       "Gruppe 1A",
			maxDistance: 0,
			minMatches:  1,
			maxMatches:  1,
			firstMatch:  "Gruppe 1A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := resolver.findSimilarGroups(tt.input, tt.maxDistance)

			assert.GreaterOrEqual(t, len(suggestions), tt.minMatches, "Too few suggestions")
			assert.LessOrEqual(t, len(suggestions), tt.maxMatches, "Too many suggestions")
			assert.LessOrEqual(t, len(suggestions), 3, "Should return max 3 suggestions")

			if len(suggestions) > 0 && tt.firstMatch != "" {
				// First suggestion should be the closest match
				assert.Contains(t, suggestions, tt.firstMatch, "Expected match not in suggestions")
			}
		})
	}
}

// Helper function
func int64Ptr(i int64) *int64 {
	return &i
}

package suggestions

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPost_Validate_Success(t *testing.T) {
	post := &Post{
		Title:       "Valid Title",
		Description: "Valid description",
		AuthorID:    1,
		Status:      StatusOpen,
	}

	err := post.Validate()
	require.NoError(t, err)
}

func TestPost_Validate_TrimsWhitespace(t *testing.T) {
	post := &Post{
		Title:       "  Trimmed Title  ",
		Description: "  Trimmed Description  ",
		AuthorID:    1,
		Status:      StatusOpen,
	}

	err := post.Validate()
	require.NoError(t, err)
	assert.Equal(t, "Trimmed Title", post.Title)
	assert.Equal(t, "Trimmed Description", post.Description)
}

func TestPost_Validate_EmptyTitle(t *testing.T) {
	post := &Post{
		Title:       "",
		Description: "Some description",
		AuthorID:    1,
		Status:      StatusOpen,
	}

	err := post.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "title is required")
}

func TestPost_Validate_WhitespaceOnlyTitle(t *testing.T) {
	post := &Post{
		Title:       "   ",
		Description: "Some description",
		AuthorID:    1,
		Status:      StatusOpen,
	}

	err := post.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "title is required")
}

func TestPost_Validate_TitleTooLong(t *testing.T) {
	post := &Post{
		Title:       strings.Repeat("a", 201),
		Description: "Some description",
		AuthorID:    1,
		Status:      StatusOpen,
	}

	err := post.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "title must not exceed 200 characters")
}

func TestPost_Validate_TitleExactly200(t *testing.T) {
	post := &Post{
		Title:       strings.Repeat("a", 200),
		Description: "Some description",
		AuthorID:    1,
		Status:      StatusOpen,
	}

	err := post.Validate()
	require.NoError(t, err)
}

func TestPost_Validate_EmptyDescription(t *testing.T) {
	post := &Post{
		Title:       "Valid Title",
		Description: "",
		AuthorID:    1,
		Status:      StatusOpen,
	}

	err := post.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description is required")
}

func TestPost_Validate_DescriptionTooLong(t *testing.T) {
	post := &Post{
		Title:       "Valid Title",
		Description: strings.Repeat("a", 5001),
		AuthorID:    1,
		Status:      StatusOpen,
	}

	err := post.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description must not exceed 5000 characters")
}

func TestPost_Validate_DescriptionExactly5000(t *testing.T) {
	post := &Post{
		Title:       "Valid Title",
		Description: strings.Repeat("a", 5000),
		AuthorID:    1,
		Status:      StatusOpen,
	}

	err := post.Validate()
	require.NoError(t, err)
}

func TestPost_Validate_ZeroAuthorID(t *testing.T) {
	post := &Post{
		Title:       "Valid Title",
		Description: "Some description",
		AuthorID:    0,
		Status:      StatusOpen,
	}

	err := post.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "author ID is required")
}

func TestPost_Validate_NegativeAuthorID(t *testing.T) {
	post := &Post{
		Title:       "Valid Title",
		Description: "Some description",
		AuthorID:    -1,
		Status:      StatusOpen,
	}

	err := post.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "author ID is required")
}

func TestPost_Validate_InvalidStatus(t *testing.T) {
	post := &Post{
		Title:       "Valid Title",
		Description: "Some description",
		AuthorID:    1,
		Status:      "invalid",
	}

	err := post.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestPost_Validate_AllStatuses(t *testing.T) {
	statuses := []string{StatusOpen, StatusPlanned, StatusDone, StatusRejected}
	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			post := &Post{
				Title:       "Valid Title",
				Description: "Valid description",
				AuthorID:    1,
				Status:      status,
			}
			err := post.Validate()
			require.NoError(t, err)
		})
	}
}

func TestIsValidStatus(t *testing.T) {
	tests := []struct {
		status string
		valid  bool
	}{
		{StatusOpen, true},
		{StatusPlanned, true},
		{StatusDone, true},
		{StatusRejected, true},
		{"", false},
		{"invalid", false},
		{"OPEN", false},
		{"Open", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			assert.Equal(t, tt.valid, IsValidStatus(tt.status))
		})
	}
}

func TestPost_TableName(t *testing.T) {
	post := &Post{}
	assert.Equal(t, "suggestions.posts", post.TableName())
}

func TestPost_GetID(t *testing.T) {
	post := &Post{}
	post.ID = 42
	assert.Equal(t, int64(42), post.GetID())
}

func TestPost_GetCreatedAt(t *testing.T) {
	now := time.Now()
	post := &Post{}
	post.CreatedAt = now
	assert.Equal(t, now, post.GetCreatedAt())
}

func TestPost_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	post := &Post{}
	post.UpdatedAt = now
	assert.Equal(t, now, post.GetUpdatedAt())
}

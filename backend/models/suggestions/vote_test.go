package suggestions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVote_Validate_Success(t *testing.T) {
	vote := &Vote{
		PostID:    1,
		VoterID:   2,
		Direction: DirectionUp,
	}

	err := vote.Validate()
	require.NoError(t, err)
}

func TestVote_Validate_DownDirection(t *testing.T) {
	vote := &Vote{
		PostID:    1,
		VoterID:   2,
		Direction: DirectionDown,
	}

	err := vote.Validate()
	require.NoError(t, err)
}

func TestVote_Validate_ZeroPostID(t *testing.T) {
	vote := &Vote{
		PostID:    0,
		VoterID:   2,
		Direction: DirectionUp,
	}

	err := vote.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "post ID is required")
}

func TestVote_Validate_NegativePostID(t *testing.T) {
	vote := &Vote{
		PostID:    -1,
		VoterID:   2,
		Direction: DirectionUp,
	}

	err := vote.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "post ID is required")
}

func TestVote_Validate_ZeroVoterID(t *testing.T) {
	vote := &Vote{
		PostID:    1,
		VoterID:   0,
		Direction: DirectionUp,
	}

	err := vote.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "voter ID is required")
}

func TestVote_Validate_InvalidDirection(t *testing.T) {
	vote := &Vote{
		PostID:    1,
		VoterID:   2,
		Direction: "sideways",
	}

	err := vote.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "direction must be 'up' or 'down'")
}

func TestVote_Validate_EmptyDirection(t *testing.T) {
	vote := &Vote{
		PostID:    1,
		VoterID:   2,
		Direction: "",
	}

	err := vote.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "direction must be 'up' or 'down'")
}

func TestIsValidDirection(t *testing.T) {
	tests := []struct {
		direction string
		valid     bool
	}{
		{DirectionUp, true},
		{DirectionDown, true},
		{"", false},
		{"sideways", false},
		{"UP", false},
		{"Down", false},
	}

	for _, tt := range tests {
		t.Run(tt.direction, func(t *testing.T) {
			assert.Equal(t, tt.valid, IsValidDirection(tt.direction))
		})
	}
}

func TestVote_TableName(t *testing.T) {
	vote := &Vote{}
	assert.Equal(t, "suggestions.votes", vote.TableName())
}

func TestVote_GetID(t *testing.T) {
	vote := &Vote{}
	vote.ID = 99
	assert.Equal(t, int64(99), vote.GetID())
}

func TestVote_GetCreatedAt(t *testing.T) {
	now := time.Now()
	vote := &Vote{}
	vote.CreatedAt = now
	assert.Equal(t, now, vote.GetCreatedAt())
}

func TestVote_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	vote := &Vote{}
	vote.UpdatedAt = now
	assert.Equal(t, now, vote.GetUpdatedAt())
}

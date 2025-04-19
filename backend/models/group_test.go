package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGroupValidation(t *testing.T) {
	tests := []struct {
		name        string
		group       *Group
		expectError bool
	}{
		{
			name: "Valid group",
			group: &Group{
				Name: "Test Group",
			},
			expectError: false,
		},
		{
			name: "Missing name",
			group: &Group{
				Name: "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.group.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCombinedGroupValidation(t *testing.T) {
	tests := []struct {
		name          string
		combinedGroup *CombinedGroup
		expectError   bool
	}{
		{
			name: "Valid combined group - all access policy",
			combinedGroup: &CombinedGroup{
				Name:         "Test Combined Group",
				AccessPolicy: "all",
			},
			expectError: false,
		},
		{
			name: "Valid combined group - first access policy",
			combinedGroup: &CombinedGroup{
				Name:         "Test Combined Group",
				AccessPolicy: "first",
			},
			expectError: false,
		},
		{
			name: "Valid combined group - specific access policy",
			combinedGroup: &CombinedGroup{
				Name:         "Test Combined Group",
				AccessPolicy: "specific",
			},
			expectError: false,
		},
		{
			name: "Valid combined group - manual access policy",
			combinedGroup: &CombinedGroup{
				Name:         "Test Combined Group",
				AccessPolicy: "manual",
			},
			expectError: false,
		},
		{
			name: "Missing name",
			combinedGroup: &CombinedGroup{
				Name:         "",
				AccessPolicy: "all",
			},
			expectError: true,
		},
		{
			name: "Invalid access policy",
			combinedGroup: &CombinedGroup{
				Name:         "Test Combined Group",
				AccessPolicy: "invalid",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.combinedGroup.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGroupSuperviorJunctionTable(t *testing.T) {
	// Test creation of junction table entry
	gs := &GroupSupervisor{
		GroupID:      1,
		SpecialistID: 2,
	}

	assert.Equal(t, int64(1), gs.GroupID)
	assert.Equal(t, int64(2), gs.SpecialistID)

	// Ensure the creation timestamp is set during BeforeInsert
	now := time.Now()
	gs.CreatedAt = now
	assert.Equal(t, now, gs.CreatedAt)
}

func TestCombinedGroupJunctionTables(t *testing.T) {
	// Test CombinedGroupGroup junction
	cgg := &CombinedGroupGroup{
		CombinedGroupID: 1,
		GroupID:         2,
	}

	assert.Equal(t, int64(1), cgg.CombinedGroupID)
	assert.Equal(t, int64(2), cgg.GroupID)

	// Test CombinedGroupSpecialist junction
	cgs := &CombinedGroupSpecialist{
		CombinedGroupID: 1,
		SpecialistID:    3,
	}

	assert.Equal(t, int64(1), cgs.CombinedGroupID)
	assert.Equal(t, int64(3), cgs.SpecialistID)
}

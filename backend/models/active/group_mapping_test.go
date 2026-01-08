package active

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestGroupMappingValidate(t *testing.T) {
	tests := []struct {
		name         string
		groupMapping *GroupMapping
		wantErr      bool
	}{
		{
			name: "Valid group mapping",
			groupMapping: &GroupMapping{
				ActiveCombinedGroupID: 1,
				ActiveGroupID:         1,
			},
			wantErr: false,
		},
		{
			name: "Missing active combined group ID",
			groupMapping: &GroupMapping{
				ActiveGroupID: 1,
			},
			wantErr: true,
		},
		{
			name: "Missing active group ID",
			groupMapping: &GroupMapping{
				ActiveCombinedGroupID: 1,
			},
			wantErr: true,
		},
		{
			name: "Invalid active combined group ID",
			groupMapping: &GroupMapping{
				ActiveCombinedGroupID: -1,
				ActiveGroupID:         1,
			},
			wantErr: true,
		},
		{
			name: "Invalid active group ID",
			groupMapping: &GroupMapping{
				ActiveCombinedGroupID: 1,
				ActiveGroupID:         0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.groupMapping.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("GroupMapping.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGroupMappingTableName(t *testing.T) {
	groupMapping := &GroupMapping{}
	want := "active.group_mappings"

	if got := groupMapping.TableName(); got != want {
		t.Errorf("GroupMapping.TableName() = %v, want %v", got, want)
	}
}

func TestGroupMapping_EntityInterface(t *testing.T) {
	now := time.Now()
	groupMapping := &GroupMapping{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		ActiveCombinedGroupID: 1,
		ActiveGroupID:         2,
	}

	t.Run("GetID", func(t *testing.T) {
		got := groupMapping.GetID()
		if got != int64(123) {
			t.Errorf("GroupMapping.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := groupMapping.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("GroupMapping.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := groupMapping.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("GroupMapping.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}

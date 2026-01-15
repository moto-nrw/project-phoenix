package active

import (
	"testing"
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

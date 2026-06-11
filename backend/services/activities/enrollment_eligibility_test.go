// Package activities_test tests the activities service layer.
package activities_test

import (
	"testing"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/services/activities"
)

// TestCanStudentJoinGroup covers the enrollment eligibility decision after it
// was moved off the Group model into the service (Rule 12). The method is a
// pure boolean over its arguments and touches no service state, so it runs
// without a database.
func TestCanStudentJoinGroup(t *testing.T) {
	svc := &activities.Service{}

	tests := []struct {
		name                   string
		group                  *activitiesModels.Group
		currentEnrollmentCount int
		want                   bool
	}{
		{
			name:                   "can join (open and has spots)",
			group:                  &activitiesModels.Group{IsOpen: true, MaxParticipants: 10},
			currentEnrollmentCount: 5,
			want:                   true,
		},
		{
			name:                   "cannot join (closed)",
			group:                  &activitiesModels.Group{IsOpen: false, MaxParticipants: 10},
			currentEnrollmentCount: 5,
			want:                   false,
		},
		{
			name:                   "cannot join (open but full)",
			group:                  &activitiesModels.Group{IsOpen: true, MaxParticipants: 10},
			currentEnrollmentCount: 10,
			want:                   false,
		},
		{
			name:                   "cannot join (closed and full)",
			group:                  &activitiesModels.Group{IsOpen: false, MaxParticipants: 10},
			currentEnrollmentCount: 10,
			want:                   false,
		},
		{
			name:                   "nil group cannot be joined",
			group:                  nil,
			currentEnrollmentCount: 0,
			want:                   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.CanStudentJoinGroup(tt.group, tt.currentEnrollmentCount); got != tt.want {
				t.Errorf("CanStudentJoinGroup() = %v, want %v", got, tt.want)
			}
		})
	}
}

package common

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

type failingSnapshotPersonService struct {
	userService.PersonService
	err error
}

func (s failingSnapshotPersonService) GetByIDs(context.Context, []int64) (map[int64]*userModels.Person, error) {
	return nil, s.err
}

type failingSnapshotEducationService struct {
	educationService.Service
	err error
}

func (s failingSnapshotEducationService) GetGroupsByIDs(context.Context, []int64) (map[int64]*educationModels.Group, error) {
	return nil, s.err
}

type failingSnapshotActiveService struct {
	activeService.Service
	err error
}

func (s failingSnapshotActiveService) GetPresenceMode(context.Context) string {
	return PresenceModeDetailed
}

func (s failingSnapshotActiveService) GetStudentsAttendanceStatuses(context.Context, []int64) (map[int64]*activeService.AttendanceStatus, error) {
	return nil, s.err
}

func TestLoadStudentDataSnapshotStrictPropagatesSubLoadErrors(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("snapshot source unavailable")

	tests := []struct {
		name         string
		personSvc    userService.PersonService
		educationSvc educationService.Service
		activeSvc    activeService.Service
		studentIDs   []int64
		personIDs    []int64
		groupIDs     []int64
	}{
		{
			name:      "persons",
			personSvc: failingSnapshotPersonService{err: wantErr},
			personIDs: []int64{101},
		},
		{
			name:         "groups",
			educationSvc: failingSnapshotEducationService{err: wantErr},
			groupIDs:     []int64{201},
		},
		{
			name:       "locations",
			activeSvc:  failingSnapshotActiveService{err: wantErr},
			studentIDs: []int64{301},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot, err := LoadStudentDataSnapshotStrict(
				context.Background(),
				tt.personSvc,
				tt.educationSvc,
				tt.activeSvc,
				tt.studentIDs,
				tt.personIDs,
				tt.groupIDs,
			)

			require.ErrorIs(t, err, wantErr)
			require.Nil(t, snapshot)
		})
	}
}

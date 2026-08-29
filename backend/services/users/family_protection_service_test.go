package users

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

type familyProtectionRepoStub struct {
	current map[int64]*userModels.FamilyProtectionEvent
	created []*userModels.FamilyProtectionEvent
	loaded  []int64
}

func (s *familyProtectionRepoStub) CurrentForStudents(_ context.Context, studentIDs []int64) (map[int64]*userModels.FamilyProtectionEvent, error) {
	s.loaded = append([]int64(nil), studentIDs...)
	return s.current, nil
}

func (s *familyProtectionRepoStub) Create(_ context.Context, event *userModels.FamilyProtectionEvent) error {
	s.created = append(s.created, event)
	return nil
}

type familyProtectionStudentStub struct {
	student *userModels.Student
}

func (s *familyProtectionStudentStub) FindByIDForUpdate(context.Context, int64) (*userModels.Student, error) {
	return s.student, nil
}

func familyProtectionContext(permissions ...string) context.Context {
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, jwt.AppClaims{ID: 77})
	return context.WithValue(ctx, jwt.CtxPermissions, permissions)
}

func TestFamilyProtectionServiceRequiresConfigurationPermission(t *testing.T) {
	t.Parallel()

	repo := &familyProtectionRepoStub{}
	service := NewFamilyProtectionService(repo, &familyProtectionStudentStub{student: &userModels.Student{}})

	_, err := service.Set(familyProtectionContext("users:update"), SetFamilyProtectionInput{
		StudentID: 10, Enabled: true, Reason: "Schutz nötig", ActorAccountID: 77,
	})

	require.ErrorIs(t, err, ErrFamilyProtectionForbidden)
	assert.Empty(t, repo.created)
}

func TestFamilyProtectionServiceValidatesAndDeduplicatesStudentIDs(t *testing.T) {
	t.Parallel()

	repo := &familyProtectionRepoStub{current: map[int64]*userModels.FamilyProtectionEvent{}}
	service := NewFamilyProtectionService(repo, &familyProtectionStudentStub{})

	_, err := service.Current(context.Background(), []int64{10, 10, 11})
	require.NoError(t, err)
	assert.Equal(t, []int64{10, 11}, repo.loaded)

	_, err = service.Current(context.Background(), []int64{0})
	require.ErrorIs(t, err, ErrFamilyProtectionInvalid)
}

func TestFamilyProtectionServiceAppendsAuditedStateChange(t *testing.T) {
	t.Parallel()

	repo := &familyProtectionRepoStub{current: map[int64]*userModels.FamilyProtectionEvent{}}
	service := NewFamilyProtectionService(repo, &familyProtectionStudentStub{student: &userModels.Student{}})

	event, err := service.Set(familyProtectionContext("config:manage"), SetFamilyProtectionInput{
		StudentID: 10, Enabled: true, Reason: "Schutz nötig", ActorAccountID: 77,
	})

	require.NoError(t, err)
	require.Len(t, repo.created, 1)
	assert.Same(t, event, repo.created[0])
	assert.Equal(t, int64(10), event.StudentID)
	assert.True(t, event.Enabled)
	assert.Equal(t, "Schutz nötig", event.Reason)
	assert.Equal(t, int64(77), event.ActorAccountID)
}

func TestFamilyProtectionServiceDoesNotAppendUnchangedState(t *testing.T) {
	t.Parallel()

	existing := &userModels.FamilyProtectionEvent{StudentID: 10, Enabled: true}
	repo := &familyProtectionRepoStub{current: map[int64]*userModels.FamilyProtectionEvent{10: existing}}
	service := NewFamilyProtectionService(repo, &familyProtectionStudentStub{student: &userModels.Student{}})

	event, err := service.Set(familyProtectionContext("config:manage"), SetFamilyProtectionInput{
		StudentID: 10, Enabled: true, Reason: "noch einmal", ActorAccountID: 77,
	})

	require.NoError(t, err)
	assert.Same(t, existing, event)
	assert.Empty(t, repo.created)
}

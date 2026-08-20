// Contract tests for the Stammdaten methods of the shared PersonService mock
// (#1423). The mock's promise is exactly two things: delegate to the Fn field
// when one is set, and return zero values when it is not — a nil-Fn method
// that panicked or returned a non-nil value would break every consumer that
// stubs only the calls it exercises.
package userstest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// staffID is an in-memory argument, not a database row.
const staffID = int64(4711)

func TestPersonServiceMock_StammdatenDelegatesToFuncFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sentinel := errors.New("boom")

	var (
		gotStaffID  int64
		gotNote     string
		gotActor    int64
		gotRole     string
		gotQualLen  int
		gotEmployer *string
	)

	employmentType := "part_time"
	mock := &userstest.PersonServiceMock{
		GetStaffStammdatenFn: func(_ context.Context, id int64) (*users.StaffStammdaten, error) {
			gotStaffID = id
			return &users.StaffStammdaten{}, nil
		},
		UpdateStaffStammdatenPersonFn: func(_ context.Context, id int64, _ users.StammdatenPersonInput, _ int64, note string) error {
			gotStaffID, gotNote = id, note
			return nil
		},
		UpdateStaffStammdatenKontaktFn: func(_ context.Context, _ int64, _ users.StammdatenKontaktInput, _ int64, _ string) error {
			return sentinel
		},
		UpdateStaffStammdatenArbeitsvertragFn: func(_ context.Context, _ int64, input users.StammdatenArbeitsvertragInput, _ int64, _ string) error {
			gotEmployer = input.EmploymentType
			return nil
		},
		ReplaceStaffQualificationsFn: func(_ context.Context, _ int64, inputs []users.StammdatenQualificationInput, _ int64, _ string) error {
			gotQualLen = len(inputs)
			return nil
		},
		GetStaffFinancialMaskedFn: func(_ context.Context, _ int64, actorAccountID int64, actorRole string) (*users.StaffFinancialMasked, error) {
			gotActor, gotRole = actorAccountID, actorRole
			return &users.StaffFinancialMasked{StaffID: staffID}, nil
		},
		RevealStaffFinancialFn: func(_ context.Context, _ int64, _ int64, _ string) (*users.StaffFinancialPlain, error) {
			return nil, sentinel
		},
		UpdateStaffFinancialFn: func(_ context.Context, _ int64, _ users.StammdatenFinancialInput, changedByAccountID int64, _ string) error {
			gotActor = changedByAccountID
			return nil
		},
	}

	stammdaten, err := mock.GetStaffStammdaten(ctx, staffID)
	require.NoError(t, err)
	require.NotNil(t, stammdaten)
	assert.Equal(t, staffID, gotStaffID)

	require.NoError(t, mock.UpdateStaffStammdatenPerson(ctx, staffID, users.StammdatenPersonInput{FirstName: "Max"}, staffID, "Namensänderung"))
	assert.Equal(t, "Namensänderung", gotNote)

	assert.ErrorIs(t, mock.UpdateStaffStammdatenKontakt(ctx, staffID, users.StammdatenKontaktInput{}, staffID, ""), sentinel)

	require.NoError(t, mock.UpdateStaffStammdatenArbeitsvertrag(ctx, staffID, users.StammdatenArbeitsvertragInput{EmploymentType: &employmentType}, staffID, ""))
	require.NotNil(t, gotEmployer)
	assert.Equal(t, employmentType, *gotEmployer)

	require.NoError(t, mock.ReplaceStaffQualifications(ctx, staffID, []users.StammdatenQualificationInput{{Name: "Erste-Hilfe-Kurs"}}, staffID, ""))
	assert.Equal(t, 1, gotQualLen)

	masked, err := mock.GetStaffFinancialMasked(ctx, staffID, staffID, "admin")
	require.NoError(t, err)
	require.NotNil(t, masked)
	assert.Equal(t, staffID, gotActor)
	assert.Equal(t, "admin", gotRole)

	plain, err := mock.RevealStaffFinancial(ctx, staffID, staffID, "admin")
	assert.ErrorIs(t, err, sentinel)
	assert.Nil(t, plain)

	require.NoError(t, mock.UpdateStaffFinancial(ctx, staffID, users.StammdatenFinancialInput{}, staffID, ""))
	assert.Equal(t, staffID, gotActor)
}

func TestPersonServiceMock_StammdatenZeroValuesWithoutStubs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := &userstest.PersonServiceMock{}

	stammdaten, err := mock.GetStaffStammdaten(ctx, staffID)
	require.NoError(t, err)
	assert.Nil(t, stammdaten)

	require.NoError(t, mock.UpdateStaffStammdatenPerson(ctx, staffID, users.StammdatenPersonInput{}, staffID, ""))
	require.NoError(t, mock.UpdateStaffStammdatenKontakt(ctx, staffID, users.StammdatenKontaktInput{}, staffID, ""))
	require.NoError(t, mock.UpdateStaffStammdatenArbeitsvertrag(ctx, staffID, users.StammdatenArbeitsvertragInput{}, staffID, ""))
	require.NoError(t, mock.ReplaceStaffQualifications(ctx, staffID, nil, staffID, ""))
	require.NoError(t, mock.UpdateStaffFinancial(ctx, staffID, users.StammdatenFinancialInput{}, staffID, ""))

	masked, err := mock.GetStaffFinancialMasked(ctx, staffID, staffID, "")
	require.NoError(t, err)
	assert.Nil(t, masked)

	plain, err := mock.RevealStaffFinancial(ctx, staffID, staffID, "")
	require.NoError(t, err)
	assert.Nil(t, plain)
}

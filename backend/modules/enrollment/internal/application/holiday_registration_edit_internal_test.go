package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// editReviewStore is a review store with one received registration of one
// child. Its audit entry fails on demand: the edit then fails too, so the
// Träger transaction rolls the written details back (#5629). The embedded
// interface panics on every method the edit does not call.
type editReviewStore struct {
	HolidayReviewStore
	auditErr error
	written  int
	audited  []string
}

func (s *editReviewStore) LockHolidayRegistration(context.Context, int64, int64, int64) (string, string, string, bool, error) {
	return enrollment.HolidayRegistrationReceived, "", "", true, nil
}

func (s *editReviewStore) ReadHolidayReview(context.Context, int64, int64, int64) ([]enrollment.HolidayRegistrationDetail, map[int64]string, error) {
	return []enrollment.HolidayRegistrationDetail{{HolidayRegistrationSummary: enrollment.HolidayRegistrationSummary{
		ID: 7, Status: enrollment.HolidayRegistrationReceived, GuardianFirstName: "Eva", GuardianLastName: "Muster",
		GuardianEmail: "eva.muster@example.org", GuardianPhone: "0151 1",
		Children: []enrollment.HolidayRegistrationChild{{ID: 11, FirstName: "Lia", LastName: "Muster", BirthDate: calendar.NewDate(2018, 3, 1)}},
	}}}, map[int64]string{}, nil
}

func (s *editReviewStore) ReadHolidayContacts(context.Context, int64, int64) ([]enrollment.HolidayRegistrationContact, error) {
	return []enrollment.HolidayRegistrationContact{{Kind: enrollment.HolidayContactEmergency, Name: "Oma Muster", Phone: "05402 1"}}, nil
}

func (s *editReviewStore) ReadHolidayAnswers(context.Context, int64, int64, int64) (map[int64]enrollment.HolidayRegistrationAnswers, error) {
	return map[int64]enrollment.HolidayRegistrationAnswers{7: {FormVersion: 1, CustomData: enrollment.HolidayAnswers{},
		Children: []enrollment.HolidayChildAnswers{{ChildID: 11, CustomData: enrollment.HolidayAnswers{}}}}}, nil
}

func (s *editReviewStore) WriteHolidayRegistrationDetails(context.Context, int64, int64, enrollment.HolidayRegistrationInput, []int64, []bool) error {
	s.written++
	return nil
}

func (s *editReviewStore) AppendHolidayAudit(_ context.Context, _, _, _ int64, action string, _ any, _ int64) error {
	if s.auditErr != nil {
		return s.auditErr
	}
	s.audited = append(s.audited, action)
	return nil
}

func (s *editReviewStore) ReadHolidayHistory(context.Context, int64, int64) ([]enrollment.HolidayRegistrationEvent, error) {
	return nil, nil
}

func (s *editReviewStore) ReadHolidayProofs(context.Context, int64, int64) ([]enrollment.HolidayRegistrationProof, error) {
	return nil, nil
}

type editPeriodStore struct{ HolidayPeriodStore }

func (editPeriodStore) FindHolidayPeriod(context.Context, int64, int64, bool) (enrollment.HolidayPeriod, bool, error) {
	return enrollment.HolidayPeriod{ID: 3, RegistrationClosesOn: calendar.NewDate(2099, 1, 1)}, true, nil
}

type editFormStore struct{ HolidayFormStore }

func (editFormStore) HolidayPeriodForms(context.Context, int64, int64, []int) (map[int]enrollment.HolidayPeriodForm, error) {
	return map[int]enrollment.HolidayPeriodForm{1: {Version: 1}}, nil
}

type editCipher struct{}

func (editCipher) Seal(plaintext, _ []byte) ([]byte, error) { return plaintext, nil }
func (editCipher) Open(sealed, _ []byte) ([]byte, error)    { return sealed, nil }

func newEditReview(store *editReviewStore) *HolidayRegistrationReview {
	return NewHolidayRegistrationReview(store, editPeriodStore{}, editFormStore{}, editCipher{},
		func() calendar.Date { return calendar.NewDate(2026, 11, 3) }, HolidayPlaceDependencies{})
}

func editedPhone(phone string) enrollment.HolidayRegistrationEdit {
	return enrollment.HolidayRegistrationEdit{Guardians: &enrollment.HolidayGuardiansEdit{Guardian: enrollment.HolidayGuardianInput{
		FirstName: "Eva", LastName: "Muster", Email: "eva.muster@example.org", Phone: phone,
	}}}
}

func TestHolidayRegistrationEditFailsWithItsAuditEntry(t *testing.T) {
	t.Parallel()
	store := &editReviewStore{auditErr: errors.New("audit unavailable")}
	reviewer := enrollment.HolidayReviewer{OrganizationID: 1, AccountID: 2}
	_, err := newEditReview(store).EditHolidayRegistration(t.Context(), reviewer, 3, 7, editedPhone("0151 2"), nil)
	require.ErrorIs(t, err, store.auditErr, "without the entry the edit fails and its transaction rolls back")
}

func TestHolidayRegistrationEditWithoutChangeWritesNothing(t *testing.T) {
	t.Parallel()
	store := &editReviewStore{}
	reviewer := enrollment.HolidayReviewer{OrganizationID: 1, AccountID: 2}
	_, err := newEditReview(store).EditHolidayRegistration(t.Context(), reviewer, 3, 7, editedPhone(" 0151 1 "), nil)
	require.NoError(t, err)
	assert.Zero(t, store.written, "the same values, trimmed, are no change")
	assert.Empty(t, store.audited)

	_, err = newEditReview(store).EditHolidayRegistration(t.Context(), reviewer, 3, 7, editedPhone("0151 2"), nil)
	require.NoError(t, err)
	assert.Equal(t, 1, store.written)
	assert.Equal(t, []string{enrollment.HolidayAuditEdit}, store.audited)
}

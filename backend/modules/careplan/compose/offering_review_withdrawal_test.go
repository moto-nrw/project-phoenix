package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// offeringWithdrawalFixture is one child with a care and a lunch booking and
// its pending offering change.
type offeringWithdrawalFixture struct {
	studentID, childID, requestID, phaseID, personID int64
	care, lunch                                      careplan.CareOffering
}

func newOfferingWithdrawalFixture(t *testing.T, db *bun.DB, module *careplan.Module) offeringWithdrawalFixture {
	t.Helper()
	ctx := testpkg.Ctx(t)
	studentID, childID, _ := insertOfferingChangeFixture(t, db, ctx, testpkg.Tenant(t))
	fixture := offeringWithdrawalFixture{studentID: studentID, childID: childID}
	err := tenant.WithinTenant(ctx, mustTenantID(t, testpkg.Tenant(t)), func(txCtx context.Context) error {
		return transactionDB(t, txCtx).NewRaw(`SELECT s.person_id, c.request_id, r.phase_id FROM users.student_profiles s JOIN enrollment.request_children c ON c.created_student_id = s.id JOIN enrollment.requests r ON r.id = c.request_id WHERE s.id = ? AND c.id = ?`, studentID, childID).
			Scan(txCtx, &fixture.personID, &fixture.requestID, &fixture.phaseID)
	})
	require.NoError(t, err)
	fixture.care = createOffering(t, ctx, module, offeringFields(fixture.phaseID, "Ganztag"))
	lunch := offeringFields(fixture.phaseID, "Mittagessen")
	lunch.CountsAsCare = false
	fixture.lunch = createOffering(t, ctx, module, lunch)
	return fixture
}

func (f offeringWithdrawalFixture) request(t *testing.T, module *careplan.Module, accountID int64, offeringIDs ...int64) careplan.OfferingChangeRequest {
	t.Helper()
	offerings := make([]map[string]any, 0, len(offeringIDs))
	for _, id := range offeringIDs {
		offerings = append(offerings, map[string]any{"offering_id": id})
	}
	// A request that drops every care day needs the confirmed withdrawal.
	confirmedAt := time.Date(2030, 8, 1, 9, 0, 0, 0, time.UTC)
	payload, err := json.Marshal(map[string]any{"offerings": offerings})
	require.NoError(t, err)
	row, err := module.CreateOfferingChange(testpkg.Ctx(t), careplan.OfferingChangeRequest{
		StudentID: f.studentID, RequestChildID: f.childID, SubmittedBy: accountID, Payload: payload,
		EffectiveFrom: "2030-09-01", Status: "pending",
		CompleteWithdrawalConfirmed: true, WithdrawalConfirmedBy: &accountID, WithdrawalConfirmedAt: &confirmedAt,
	})
	require.NoError(t, err)
	return row
}

func (f offeringWithdrawalFixture) enrollment() *offeringEnrollmentFixture {
	return &offeringEnrollmentFixture{
		child:   OfferingReviewChild{ID: f.childID, RequestID: f.requestID},
		request: OfferingReviewRequest{ID: f.requestID, PhaseID: f.phaseID},
		phase:   OfferingReviewPhase{ID: f.phaseID, Start: "2030-09-01", End: "2031-07-31", SelectionMode: "optional"},
		bookings: []OfferingReviewBooking{
			{ChildID: f.childID, OfferingID: f.care.ID},
			{ChildID: f.childID, OfferingID: f.lunch.ID},
		},
	}
}

func pendingOfferingReview(t *testing.T, db *bun.DB, fixture offeringWithdrawalFixture) *careplan.OfferingReviewItem {
	t.Helper()
	query, err := NewOfferingReviews(db, func(Observation) {}, OfferingReviewDependencies{
		People: offeringDirectoryFixture{reviewDirectoryFixture{
			students: map[int64]ReviewStudent{fixture.studentID: {ID: fixture.studentID, PersonID: fixture.personID}},
			names:    map[int64]PersonName{fixture.personID: {FirstName: "Care", LastName: "Change"}},
		}},
		Enrollment: fixture.enrollment(),
		Courses:    &offeringCoursesFixture{},
		Scope:      func(context.Context) (ReviewScope, error) { return ReviewScope{SchoolWide: true}, nil },
		Today:      func() careplan.Date { return "2030-08-15" },
	})
	require.NoError(t, err)
	rows, _, err := query.ListPending(testpkg.Ctx(t), careplan.RequestQueueFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	return rows[0]
}

// The staff queue says what a decision does to the child's whole booking
// picture (#2434): a request that empties the list is a Komplett-Abmeldung,
// and one that keeps an offering names it among the untouched bookings.
func TestOfferingReviewMarksCompleteWithdrawalAndUntouchedBookings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("offering-withdrawal-%d@example.test", testpkg.UniqueSuffix()))

	t.Run("an empty selection is a Komplett-Abmeldung", func(t *testing.T) {
		fixture := newOfferingWithdrawalFixture(t, db, module)
		fixture.request(t, module, account.ID)

		review := pendingOfferingReview(t, db, fixture)
		assert.True(t, review.FullWithdrawal, "an empty offering list is a Komplett-Abmeldung")
		assert.Empty(t, review.Unchanged, "nothing stays booked after a Komplett-Abmeldung")
	})

	t.Run("a remaining lunch does not hide the Komplett-Abmeldung", func(t *testing.T) {
		fixture := newOfferingWithdrawalFixture(t, db, module)
		fixture.request(t, module, account.ID, fixture.lunch.ID)

		review := pendingOfferingReview(t, db, fixture)
		assert.True(t, review.FullWithdrawal, "non-care extras such as lunch do not hide the Komplett-Abmeldung")
	})

	t.Run("a kept care booking is untouched, not a withdrawal", func(t *testing.T) {
		fixture := newOfferingWithdrawalFixture(t, db, module)
		fixture.request(t, module, account.ID, fixture.care.ID)

		review := pendingOfferingReview(t, db, fixture)
		assert.False(t, review.FullWithdrawal, "one care offering stays booked, so this is no Komplett-Abmeldung")
		require.Len(t, review.Unchanged, 1)
		assert.Equal(t, fixture.care.ID, review.Unchanged[0].OfferingID)
		for _, entry := range review.Diff {
			assert.NotEqual(t, fixture.care.ID, entry.OfferingID, "an untouched booking does not belong in the changed lines")
		}
	})
}

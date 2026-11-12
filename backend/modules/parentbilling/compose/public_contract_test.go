package compose_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// confirmedRegistration is Enrollment's side of the port: it confirms the
// one registration it knows and reports its bookings.
type confirmedRegistration struct {
	registration parentbilling.HolidayRegistration
}

func (c confirmedRegistration) ResolveHolidayLink(context.Context, string) (parentbilling.HolidayLink, error) {
	return parentbilling.HolidayLink{}, parentbilling.ErrHolidayLinkNotFound
}

func (c confirmedRegistration) HolidayWeeks(context.Context, parentbilling.HolidayLink) ([]parentbilling.HolidayWeek, error) {
	return nil, parentbilling.ErrHolidayLinkNotFound
}

func (c confirmedRegistration) HolidayRegistration(context.Context, parentbilling.HolidayReviewer, int64, int64) (parentbilling.HolidayRegistration, error) {
	return c.registration, nil
}

func (c confirmedRegistration) ConfirmHolidayRegistration(_ context.Context, _ parentbilling.HolidayReviewer, _, _ int64, debtorNumber string) (parentbilling.HolidayRegistration, error) {
	confirmed := c.registration
	confirmed.Status = parentbilling.HolidayRegistrationConfirmed
	confirmed.DebtorNumber = debtorNumber
	return confirmed, nil
}

func (c confirmedRegistration) SetHolidayDebtorNumber(context.Context, parentbilling.HolidayReviewer, int64, int64, string) (parentbilling.HolidayRegistration, error) {
	return parentbilling.HolidayRegistration{}, parentbilling.ErrHolidayRegistrationStateConflict
}

func (c confirmedRegistration) CancelHolidayRegistration(context.Context, parentbilling.HolidayReviewer, int64, int64) (parentbilling.HolidayRegistration, error) {
	return parentbilling.HolidayRegistration{}, parentbilling.ErrHolidayRegistrationStateConflict
}

func (c confirmedRegistration) RebookHolidayChild(context.Context, parentbilling.HolidayReviewer, int64, int64, int64, []parentbilling.HolidayBookingChoice) (parentbilling.HolidayRegistration, error) {
	return parentbilling.HolidayRegistration{}, parentbilling.ErrHolidayRegistrationStateConflict
}

func (c confirmedRegistration) SetHolidayWeekCapacity(context.Context, parentbilling.HolidayReviewer, int64, int64, *int) ([]int64, error) {
	return nil, parentbilling.ErrHolidayRegistrationStateConflict
}

func (c confirmedRegistration) HoldHolidayPeriod(context.Context, parentbilling.HolidayReviewer, int64) error {
	return nil
}

func (c confirmedRegistration) RecordHolidaySiblings(context.Context, parentbilling.HolidayReviewer, int64, int64, string, []int64) error {
	return nil
}

// settingsDouble is the Träger's switch for the debtor number (#5621).
type settingsDouble struct{ asksDebtorNumber bool }

func (s settingsDouble) AsksDebtorNumber(context.Context, int64) (bool, error) {
	return s.asksDebtorNumber, nil
}

func (s settingsDouble) PaymentText(context.Context, int64) (string, error) {
	return "", nil
}

// DatevSettings has no standard revenue account; the contract does not
// release a run.
func (s settingsDouble) DatevSettings(context.Context, int64) (parentbilling.DatevSettings, error) {
	return parentbilling.DatevSettings{}, nil
}

// DebtorNumberFormat is a Träger without a format (#6675).
func (s settingsDouble) DebtorNumberFormat(context.Context, int64) (parentbilling.DebtorNumberFormat, error) {
	return parentbilling.DebtorNumberFormat{}, nil
}

func (s settingsDouble) HolidayChain(context.Context, int64) (parentbilling.HolidayChainSettings, error) {
	return parentbilling.HolidayChainSettings{
		SiblingRank: parentbilling.SiblingRankHighestContribution, CostTakeoverOrder: parentbilling.CostTakeoverAfterDiscount,
	}, nil
}

func TestPublicParentBillingContractFreezesTheHolidayContribution(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	// A Träger transaction names no school, so the context carries none.
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Vertrag Beitrag", Slug: fmt.Sprintf("traeger-vertrag-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	testpkg.OwnTraegerContributionRules(t, db, organization)
	account := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("beitrag-%d@test.local", testpkg.UniqueTestTenantID(t)), testpkg.NewTestPassword())
	var period, registration, child int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on, registration_closes_on)
		VALUES (?, 'Sommerferien', '2027-05-03', '2027-06-04') RETURNING id`, organization).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
		VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0123', NOW()) RETURNING id`, organization, period).Scan(ctx, &registration))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date)
		VALUES (?, ?, ?, 1, 'Lia', 'Muster', '2018-03-01') RETURNING id`, organization, period, registration).Scan(ctx, &child))

	week := calendar.NewDate(2027, 7, 5)
	enrollment := confirmedRegistration{registration: parentbilling.HolidayRegistration{
		ID: registration, Status: "received", BenefitsDeclared: true,
		Children: []parentbilling.HolidayChild{{ID: child, Position: 1, Bookings: []parentbilling.HolidayBooking{
			{WeekID: 1, WeekStartsOn: week, WeekEndsOn: week.AddDays(4), TimeModelID: 2, TimeModelName: "bis 16 Uhr", PriceCents: 6750},
		}}},
	}}
	rules := compose.NewContributionRules()
	contributions, _ := compose.NewHolidayContributions(enrollment, settingsDouble{})
	reviewer := parentbilling.HolidayReviewer{OrganizationID: organization, AccountID: account.ID}

	var exemption parentbilling.ContributionRule
	var confirmation parentbilling.HolidayConfirmation
	var later parentbilling.HolidayContribution
	require.NoError(t, tenant.WithinTraeger(ctx, organization, func(txCtx context.Context) error {
		var err error
		exemption, err = rules.CreateContributionRule(txCtx, organization, parentbilling.ContributionRuleInput{
			Kind: parentbilling.RuleBenefitsExemption, ValidFrom: calendar.NewDate(2027, 1, 1), Note: "  Beitragsfrei wegen Leistungsbezug  ",
		})
		if err != nil {
			return err
		}
		if confirmation, err = contributions.ConfirmHolidayRegistration(txCtx, reviewer, period, registration, true, 0, "", ""); err != nil {
			return err
		}
		if err = rules.DeleteContributionRule(txCtx, organization, exemption.ID); err != nil {
			return err
		}
		later, err = contributions.HolidayContribution(txCtx, reviewer, period, registration, nil, 0)
		return err
	}))

	require.Equal(t, parentbilling.ServiceHoliday, exemption.Service)
	require.Equal(t, "Beitragsfrei wegen Leistungsbezug", exemption.Note)
	require.Equal(t, parentbilling.HolidayRegistrationConfirmed, confirmation.Status)
	require.True(t, confirmation.Contribution.Frozen)
	require.Equal(t, int64(0), confirmation.Contribution.TotalCents)
	require.Equal(t, []parentbilling.HolidayContributionLine{
		{Kind: parentbilling.LineBase, AmountCents: 6750, WeekStartsOn: &week, WeekEndsOn: new(week.AddDays(4)), TimeModelName: "bis 16 Uhr", WeekID: 1},
		{Kind: parentbilling.LineBenefitsExemption, AmountCents: -6750, Note: "Beitragsfrei wegen Leistungsbezug"},
	}, later.Children[0].Lines, "the frozen contribution keeps its exemption after the rule is gone")
	require.Equal(t, child, later.Children[0].ChildID)
}

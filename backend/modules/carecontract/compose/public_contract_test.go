package compose_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// schoolDouble is one school of the Träger with one accepted child who
// books one offering from August on.
type schoolDouble struct {
	school   carecontract.School
	child    int64
	offering int64
	accesses *int
}

func (s schoolDouble) TraegerSchools(context.Context, int64) ([]carecontract.School, error) {
	return []carecontract.School{s.school}, nil
}

func (s schoolDouble) AcceptedApplications(context.Context, []int64) ([]carecontract.Application, error) {
	return []carecontract.Application{{
		RequestChildID: s.child, ChildFirstName: "Lina", ChildLastName: "Muster",
		GuardianFirstName: "Erika", GuardianLastName: "Muster", GuardianEmail: "erika@example.org", PhaseName: "2027/28",
	}}, nil
}

func (s schoolDouble) NamePhases(_ context.Context, offerings []carecontract.Offering) ([]carecontract.Offering, error) {
	return offerings, nil
}

func (s schoolDouble) Bookings(context.Context, []int64) (map[int64][]carecontract.Booking, error) {
	from, ended := calendar.NewDate(2027, 8, 1), calendar.NewDate(2020, 1, 1)
	return map[int64][]carecontract.Booking{s.child: {
		{CareOfferingID: s.offering, Days: []string{"wed", "mon"}, ValidFrom: &from},
		// An ended booking of another offering is no candidate.
		{CareOfferingID: s.offering + 1, Days: []string{"fri"}, ValidUntil: &ended},
	}}, nil
}

func (s schoolDouble) Offerings(context.Context, []int64) ([]carecontract.Offering, error) {
	price := 9500
	return []carecontract.Offering{{ID: s.offering, Name: "Ganztag", PriceCents: &price, Active: true}}, nil
}

func (s schoolDouble) RecordAccess(context.Context, carecontract.SchoolAccess) error {
	*s.accesses++
	return nil
}

type runtimeDouble struct{}

func (runtimeDouble) WithinSchool(_ context.Context, _ int64, fn func(context.Context) error) error {
	return fn(context.Background())
}

func (runtimeDouble) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	return tenant.WithinTraeger(ctx, organizationID, fn)
}

type letterheadDouble struct{}

func (letterheadDouble) Letterhead(context.Context, carecontract.Reviewer) (carecontract.Letterhead, error) {
	return carecontract.Letterhead{SenderText: "Ganztag im Quartier e.V.\nAm Markt 4", ContactName: "Paula Kern"}, nil
}

// postDouble records the mails and answers every one as still waiting.
type postDouble struct {
	next  int64
	mails []carecontract.ContractMail
}

func (p *postDouble) EnqueueContractMail(_ context.Context, mail carecontract.ContractMail) (int64, error) {
	p.mails = append(p.mails, mail)
	p.next++
	return p.next, nil
}

func (p *postDouble) ContractMailStates(context.Context, int64, int64) ([]carecontract.MailState, error) {
	return nil, nil
}

func TestPublicCareContractCreatesOneDraftPerChildAndOfferingAndFreezesItsPDFAtTheFirstSend(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Vertrag Kern", Slug: fmt.Sprintf("traeger-vertrag-kern-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerCareContracts(t, db, organization)
	account := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("vertrag-%d@test.local", testpkg.UniqueTestTenantID(t)), testpkg.NewTestPassword())
	reviewer := carecontract.Reviewer{OrganizationID: organization, AccountID: account.ID}
	accesses := 0
	school := schoolDouble{school: carecontract.School{ID: testpkg.UniqueTestTenantID(t), Name: "OGS Am Lindenhof"},
		child: testpkg.UniqueTestTenantID(t), offering: testpkg.UniqueTestTenantID(t), accesses: &accesses}
	intake := compose.NewIntake(compose.IntakePorts{Schools: school, Applications: school, Bookings: school, Access: school, Runtime: runtimeDouble{}})

	candidates, err := intake.Candidates(ctx, reviewer, 0)
	require.NoError(t, err)
	require.Len(t, candidates, 1, "the ended booking is no candidate")
	assert.Equal(t, []string{"mon", "wed"}, candidates[0].BookedDays)
	assert.Equal(t, calendar.NewDate(2027, 8, 1), candidates[0].EffectiveFrom)
	assert.Equal(t, 1, accesses, "the read of the applications is in the school's access log")

	_, err = intake.SaveTemplate(ctx, reviewer, school.school.ID, school.offering, carecontract.TemplateInput{
		TermKind: carecontract.TermFixedMonths, TermMonths: new(12), NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd,
	})
	require.NoError(t, err)
	_, err = intake.SaveTemplate(ctx, reviewer, testpkg.UniqueTestTenantID(t), school.offering, carecontract.TemplateInput{
		TermKind: carecontract.TermOpenEnded, NoticeTo: carecontract.NoticeMonthEnd,
	})
	require.ErrorIs(t, err, carecontract.ErrSchoolOutsideTraeger)

	created, err := intake.CreateContracts(ctx, reviewer, []carecontract.CandidateKey{candidates[0].CandidateKey})
	require.NoError(t, err)
	require.Len(t, created.Created, 1)
	again, err := intake.CreateContracts(ctx, reviewer, []carecontract.CandidateKey{candidates[0].CandidateKey})
	require.NoError(t, err)
	assert.Empty(t, again.Created)
	require.Len(t, again.Skipped, 1)
	assert.Equal(t, carecontract.SkipContractExists, again.Skipped[0].Reason)

	post := &postDouble{next: testpkg.UniqueTestTenantID(t)}
	contracts := compose.NewContracts(letterheadDouble{}, post)
	id := created.Created[0].ID
	var detail carecontract.ContractDetail
	var preview, frozen carecontract.File
	require.NoError(t, tenant.WithinTraeger(ctx, organization, func(txCtx context.Context) error {
		if detail, err = contracts.Contract(txCtx, reviewer, id); err != nil {
			return err
		}
		if preview, err = contracts.ContractDocument(txCtx, reviewer, id); err != nil {
			return err
		}
		result, err := contracts.SendContracts(txCtx, reviewer, []int64{id})
		if err != nil {
			return err
		}
		assert.Equal(t, []int64{id}, result.Sent)
		frozen, err = contracts.ContractDocument(txCtx, reviewer, id)
		return err
	}))
	assert.Equal(t, carecontract.StatusDraft, detail.Status)
	require.NotNil(t, detail.Version.EndsOn, "a term of twelve months ends")
	assert.Equal(t, calendar.NewDate(2028, 7, 31), *detail.Version.EndsOn)
	require.NotNil(t, detail.Version.PriceCents)
	assert.Equal(t, int64(9500), *detail.Version.PriceCents)
	assert.True(t, bytes.HasPrefix(preview.Data, []byte("%PDF")))
	require.Len(t, post.mails, 1)
	assert.Equal(t, fmt.Sprintf("care-contract:%d:1", id), post.mails[0].IdempotencyKey)
	assert.Equal(t, "erika@example.org", post.mails[0].Recipient)
	assert.Equal(t, post.mails[0].PDF, frozen.Data, "the stored PDF is the mailed one")

	// A sent contract is no draft any more.
	err = tenant.WithinTraeger(ctx, organization, func(txCtx context.Context) error {
		return contracts.DiscardContract(txCtx, reviewer, id)
	})
	require.ErrorIs(t, err, carecontract.ErrContractNotDraft)
}

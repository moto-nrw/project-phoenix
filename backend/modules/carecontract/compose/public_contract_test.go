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
// books three modules from August on, as at the SJR Pforzheim: early care
// and care until 14:00 on Monday, care until 16:30 on Tuesday, nothing on
// Wednesday.
type schoolDouble struct {
	school   carecontract.School
	child    int64
	offering int64 // the module until 14:00; offering+1 until 16:30, offering+2 early care
	accesses *int
	reads    *[]carecontract.SchoolAccess
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
	from, ended, shortEnd := calendar.NewDate(2027, 8, 1), calendar.NewDate(2020, 1, 1), calendar.NewDate(2028, 2, 1)
	return map[int64][]carecontract.Booking{s.child: {
		// The short module ends in January, the others run on.
		{CareOfferingID: s.offering, Days: []string{"mon"}, ValidFrom: &from, ValidUntil: &shortEnd},
		{CareOfferingID: s.offering + 1, Days: []string{"tue"}, ValidFrom: &from},
		{CareOfferingID: s.offering + 2, Days: []string{"mon"}, ValidFrom: &from},
		// An ended booking is no service.
		{CareOfferingID: s.offering, Days: []string{"fri"}, ValidUntil: &ended},
		// An offering that is no care (an AG) is not among the offerings.
		{CareOfferingID: s.offering + 3, Days: []string{"thu"}, ValidFrom: &from},
	}}, nil
}

func (s schoolDouble) Offerings(context.Context, []int64) ([]carecontract.Offering, error) {
	short, long, early := 5000, 7000, 2000
	days := func(end string) map[string]string {
		return map[string]string{"mon": end, "tue": end, "wed": end, "thu": end, "fri": end}
	}
	return []carecontract.Offering{
		{ID: s.offering, Name: "Kurzbetreuung", PriceCents: &short, Active: true, EndTimes: days("14:00")},
		{ID: s.offering + 1, Name: "Ganztag", PriceCents: &long, Active: true, EndTimes: days("16:30")},
		{ID: s.offering + 2, Name: "Frühbetreuung", PriceCents: &early, Active: true, EndTimes: days("08:00")},
	}, nil
}

func (s schoolDouble) RecordAccess(_ context.Context, access carecontract.SchoolAccess) error {
	*s.accesses++
	if s.reads != nil {
		*s.reads = append(*s.reads, access)
	}
	return nil
}

// ConclusionMode keeps the school on Weg a: the Träger-Büro sends (#4680).
func (s schoolDouble) ConclusionMode(context.Context, int64) (string, error) {
	return carecontract.ModeSendLater, nil
}

type runtimeDouble struct{}

func (runtimeDouble) WithinSchool(_ context.Context, _ int64, fn func(context.Context) error) error {
	return fn(context.Background())
}

func (runtimeDouble) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	return tenant.WithinTraeger(ctx, organizationID, fn)
}

type letterheadDouble struct{ missing bool }

func (h letterheadDouble) Letterhead(context.Context, carecontract.Reviewer) (carecontract.Letterhead, error) {
	if h.missing {
		return carecontract.Letterhead{}, nil
	}
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

func (p *postDouble) ContractMailBlock(context.Context, int64, string, string) (string, error) {
	return "", nil
}

func (p *postDouble) ContractMailStates(context.Context, int64, int64) ([]carecontract.MailState, error) {
	return nil, nil
}

func TestPublicCareContractBooksModulesPerWeekdayInOneDraftAndFreezesItsPDFAtTheFirstSend(t *testing.T) {
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
	intake := compose.NewIntake(compose.IntakePorts{Schools: school, Applications: school, Bookings: school, Access: school, Runtime: runtimeDouble{}, Modes: school, Mail: noMailBlocks{}})

	shortLast := calendar.NewDate(2028, 1, 31)
	candidates, err := intake.Candidates(ctx, reviewer, 0)
	require.NoError(t, err)
	require.Len(t, candidates, 1, "one candidate per child, over all its modules")
	assert.Equal(t, []carecontract.Service{
		{Weekday: "mon", CareOfferingID: school.offering + 2, ModuleName: "Frühbetreuung", EndsAt: "08:00"},
		{Weekday: "mon", CareOfferingID: school.offering, ModuleName: "Kurzbetreuung", EndsAt: "14:00", LastDay: &shortLast},
		{Weekday: "tue", CareOfferingID: school.offering + 1, ModuleName: "Ganztag", EndsAt: "16:30"},
	}, candidates[0].Services, "per weekday the modules with their end times; no ended booking, no AG")
	assert.Equal(t, calendar.NewDate(2027, 8, 1), candidates[0].EffectiveFrom)
	assert.Equal(t, carecontract.SkipTemplateMissing, candidates[0].TemplateIssue)
	assert.Equal(t, 1, accesses, "the read of the applications is in the school's access log")

	twelveMonths := carecontract.TemplateInput{
		TermKind: carecontract.TermFixedMonths, TermMonths: new(12), NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd,
	}
	for _, offering := range []int64{school.offering, school.offering + 1} {
		_, err = intake.SaveTemplate(ctx, reviewer, school.school.ID, offering, twelveMonths)
		require.NoError(t, err)
	}
	_, err = intake.SaveTemplate(ctx, reviewer, school.school.ID, school.offering+2, carecontract.TemplateInput{
		TermKind: carecontract.TermOpenEnded, NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd,
	})
	require.NoError(t, err)
	_, err = intake.SaveTemplate(ctx, reviewer, testpkg.UniqueTestTenantID(t), school.offering, carecontract.TemplateInput{
		TermKind: carecontract.TermOpenEnded, NoticeTo: carecontract.NoticeMonthEnd,
	})
	require.ErrorIs(t, err, carecontract.ErrSchoolOutsideTraeger)

	differing, err := intake.CreateContracts(ctx, reviewer, []carecontract.CandidateKey{candidates[0].CandidateKey})
	require.NoError(t, err)
	assert.Empty(t, differing.Created, "one contract has one set of conditions")
	require.Len(t, differing.Skipped, 1)
	assert.Equal(t, carecontract.SkipTemplatesDiffer, differing.Skipped[0].Reason)

	_, err = intake.SaveTemplate(ctx, reviewer, school.school.ID, school.offering+2, twelveMonths)
	require.NoError(t, err)
	created, err := intake.CreateContracts(ctx, reviewer, []carecontract.CandidateKey{candidates[0].CandidateKey})
	require.NoError(t, err)
	require.Len(t, created.Created, 1)
	assert.Equal(t, []string{"Frühbetreuung", "Kurzbetreuung", "Ganztag"}, created.Created[0].ModuleNames)
	again, err := intake.CreateContracts(ctx, reviewer, []carecontract.CandidateKey{candidates[0].CandidateKey})
	require.NoError(t, err)
	assert.Empty(t, again.Created)
	require.Len(t, again.Skipped, 1)
	assert.Equal(t, carecontract.SkipContractExists, again.Skipped[0].Reason)

	post := &postDouble{next: testpkg.UniqueTestTenantID(t)}
	reads := []carecontract.SchoolAccess{}
	school.reads = &reads
	ports := compose.ContractPorts{Letterheads: letterheadDouble{}, Post: post, Runtime: runtimeDouble{}, Access: school}
	contracts := compose.NewContracts(ports)
	id := created.Created[0].ID
	// Missing identity cannot freeze an anonymous PDF or enqueue a mail.
	ports.Letterheads = letterheadDouble{missing: true}
	unnamed := compose.NewContracts(ports)
	_, previewErr := unnamed.ContractDocument(ctx, reviewer, id)
	var invalid *carecontract.ValidationError
	assert.ErrorAs(t, previewErr, &invalid)
	require.NoError(t, tenant.WithinTraeger(ctx, organization, func(txCtx context.Context) error {
		refused, sendErr := unnamed.SendContracts(txCtx, reviewer, []int64{id})
		assert.NoError(t, sendErr)
		assert.Empty(t, refused.Sent)
		assert.Equal(t, []carecontract.SendSkip{{ID: id, Reason: carecontract.SkipPartnerMissing}}, refused.Skipped)
		return nil
	}))
	assert.Empty(t, post.mails)
	// Every read of a contract writes into its school's access log
	// (ADR 0088 section 7): the list, the detail and the PDF.
	list, err := contracts.ListContracts(ctx, reviewer, carecontract.ContractFilter{})
	require.NoError(t, err)
	require.Len(t, list.Contracts, 1)
	detail, err := contracts.Contract(ctx, reviewer, id)
	require.NoError(t, err)
	preview, err := contracts.ContractDocument(ctx, reviewer, id)
	require.NoError(t, err)
	require.NoError(t, tenant.WithinTraeger(ctx, organization, func(txCtx context.Context) error {
		result, err := contracts.SendContracts(txCtx, reviewer, []int64{id})
		if err != nil {
			return err
		}
		assert.Equal(t, []int64{id}, result.Sent)
		return nil
	}))
	frozen, err := contracts.ContractDocument(ctx, reviewer, id)
	require.NoError(t, err)
	views := []string{}
	for _, read := range reads {
		assert.Equal(t, carecontract.SchoolAccessContracts, read.Data)
		assert.Equal(t, []int64{id}, read.ContractIDs)
		views = append(views, read.View)
	}
	assert.Equal(t, []string{"care_contract_list", "care_contract_detail", "care_contract_document", "care_contract_document"},
		views, "a refused preview showed nothing and logs nothing")
	assert.Equal(t, carecontract.StatusDraft, detail.Status)
	assert.False(t, detail.DocumentFrozen, "a missing partner did not freeze an anonymous PDF")
	require.NotNil(t, detail.Version.EndsOn, "a term of twelve months ends")
	assert.Equal(t, calendar.NewDate(2028, 7, 31), *detail.Version.EndsOn)
	assert.Equal(t, candidates[0].Services, detail.Version.Services, "the version keeps the services")
	assert.Nil(t, detail.Version.PriceCents, "no sum of whole-month prices over modules booked on single days")
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

// noMailBlocks holds no address back.
type noMailBlocks struct{}

func (noMailBlocks) ContractMailBlocks(context.Context, int64, []string) (map[string]string, error) {
	return nil, nil
}

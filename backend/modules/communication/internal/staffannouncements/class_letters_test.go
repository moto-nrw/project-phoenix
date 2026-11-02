package announcement_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	announcement "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// classLetterSettings turns every switch on: parent news and the push
// dispatch both have to be on for a letter to go out.
func classLetterSettings() *settingsDouble {
	return &settingsDouble{
		ResolveBoolFn:          func(context.Context, string) (bool, error) { return true, nil },
		ResolveBoolForTenantFn: func(context.Context, int64, string) (bool, error) { return true, nil },
	}
}

func classLetterInput(class string) announcement.Input {
	return announcement.Input{
		Title:   "Wandertag am Freitag",
		Body:    "Bitte geben Sie Ihrem Kind feste Schuhe mit.",
		Targets: []announcement.TargetInput{{TargetType: announcement.StaffAnnouncementTargetClass, RefText: &class}},
		Sender:  &announcement.ClassLetterSender{Name: " Lena Lehrerin ", SchoolClass: "1a"},
	}
}

// TestClassLetterKeepsItsSenderAndBelongsToItsAuthor pins #4800: a class
// letter stores the Lehrkraft and the class, only its author changes or
// sends it, the OGS reads it, and moto schule never reaches an OGS letter.
func TestClassLetterKeepsItsSenderAndBelongsToItsAuthor(t *testing.T) {
	t.Parallel()
	svc, db, _ := buildNoticeService(t, classLetterSettings())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	author := chain.AccountID
	asAuthor := func(c context.Context) context.Context { return communication.WithClassLetterAuthor(c, author) }
	asOther := func(c context.Context) context.Context { return communication.WithClassLetterAuthor(c, author+1) }
	inTenant := func(fn func(context.Context) error) error {
		return tenant.WithTenantTx(ctx, db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error { return fn(txCtx) })
	}

	var letter *announcement.StaffParentAnnouncement
	require.NoError(t, inTenant(func(c context.Context) error {
		var err error
		letter, err = svc.Create(asAuthor(c), author, classLetterInput("1A"))
		return err
	}))
	require.NotNil(t, letter.SenderName)
	require.NotNil(t, letter.SenderSchoolClass)
	assert.Equal(t, "Lena Lehrerin", *letter.SenderName, "the sender is trimmed")
	assert.Equal(t, "1a", *letter.SenderSchoolClass)
	require.Len(t, letter.Targets, 1)
	require.NotNil(t, letter.Targets[0].TargetRefText)
	assert.Equal(t, "1a", *letter.Targets[0].TargetRefText, "the target is spelled like the sender class")

	var ogs *announcement.StaffParentAnnouncement
	require.NoError(t, inTenant(func(c context.Context) error {
		class := "1a"
		var err error
		ogs, err = svc.Create(c, author, announcement.Input{Title: "OGS", Body: "Text",
			Targets: []announcement.TargetInput{{TargetType: announcement.StaffAnnouncementTargetClass, RefText: &class}}})
		return err
	}))
	assert.Nil(t, ogs.SenderSchoolClass, "an OGS announcement has no sender")

	for name, write := range map[string]func(context.Context, int64) error{
		"update": func(c context.Context, id int64) error {
			_, err := svc.Update(c, id, classLetterInput("1a"))
			return err
		},
		"publish":   func(c context.Context, id int64) error { _, err := svc.Publish(c, id); return err },
		"unpublish": func(c context.Context, id int64) error { _, err := svc.Unpublish(c, id); return err },
		"delete":    func(c context.Context, id int64) error { return svc.Delete(c, id) },
		"remind":    func(c context.Context, id int64) error { _, err := svc.RemindOutstanding(c, id); return err },
		"resend":    func(c context.Context, id int64) error { _, err := svc.ResendFailedEmails(c, id); return err },
	} {
		t.Run(name, func(t *testing.T) {
			assert.ErrorIs(t, inTenant(func(c context.Context) error { return write(c, letter.ID) }),
				announcement.ErrClassLetterReadOnly, "the OGS reads a class letter only")
			assert.ErrorIs(t, inTenant(func(c context.Context) error { return write(asOther(c), letter.ID) }),
				announcement.ErrClassLetterReadOnly, "another Lehrkraft does not change it")
			assert.ErrorIs(t, inTenant(func(c context.Context) error { return write(asAuthor(c), ogs.ID) }),
				announcement.ErrNotFound, "moto schule never reaches an OGS announcement")
		})
	}

	var published *announcement.StaffParentAnnouncement
	require.NoError(t, inTenant(func(c context.Context) error {
		var err error
		published, err = svc.Publish(asAuthor(c), letter.ID)
		return err
	}), "the author sends her letter")
	assert.True(t, published.IsPublished())
	require.NotNil(t, published.SenderName)
	assert.Equal(t, "Lena Lehrerin", *published.SenderName)
}

// TestClassLetterShape pins what moto schule may send (#4800): exactly the
// sender's class, no poll, the portal audience only, and always a sender.
func TestClassLetterShape(t *testing.T) {
	t.Parallel()
	svc, db, _ := buildNoticeService(t, classLetterSettings())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	create := func(in announcement.Input) error {
		return tenant.WithTenantTx(ctx, db, chain.TenantID, func(c context.Context, _ bun.Tx) error {
			_, err := svc.Create(communication.WithClassLetterAuthor(c, chain.AccountID), chain.AccountID, in)
			return err
		})
	}
	other := "2b"
	for name, in := range map[string]func(*announcement.Input){
		"another class": func(in *announcement.Input) { in.Targets[0].RefText = &other },
		"two targets": func(in *announcement.Input) {
			in.Targets = append(in.Targets, announcement.TargetInput{TargetType: announcement.StaffAnnouncementTargetSchoolAll})
		},
		"school-wide": func(in *announcement.Input) {
			in.Targets = []announcement.TargetInput{{TargetType: announcement.StaffAnnouncementTargetSchoolAll}}
		},
		"poll": func(in *announcement.Input) {
			in.ResponseType = announcement.StaffParentAnnouncementResponseSingleChoice
			in.Options = []string{"Ja", "Nein"}
		},
		"all contacts": func(in *announcement.Input) {
			in.DeliveryMode = announcement.StaffParentAnnouncementDeliveryLetter
			in.EmailAudience = announcement.StaffEmailAudienceAllContacts
			in.SendEmail = true
		},
		"no sender":  func(in *announcement.Input) { in.Sender = nil },
		"blank name": func(in *announcement.Input) { in.Sender.Name = " " },
	} {
		t.Run(name, func(t *testing.T) {
			input := classLetterInput("1a")
			in(&input)
			assert.ErrorIs(t, create(input), announcement.ErrValidation)
		})
	}
	letter := classLetterInput("1a")
	letter.DeliveryMode = announcement.StaffParentAnnouncementDeliveryLetter
	assert.NoError(t, create(letter), "an Elternbrief with read confirmation is a class letter")
}

// TestClassLetterListQueryBudgetStaysFlat pins the list behind
// GET /school/parent-letters (#4800): the school's announcement list reads
// the rows and their targets in one batch each, however many letters exist.
func TestClassLetterListQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	svc, db, _ := buildNoticeService(t, classLetterSettings())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	write := func(n int) {
		for range n {
			require.NoError(t, tenant.WithTenantTx(ctx, db, chain.TenantID, func(c context.Context, _ bun.Tx) error {
				_, err := svc.Create(communication.WithClassLetterAuthor(c, chain.AccountID), chain.AccountID, classLetterInput("1a"))
				return err
			}))
		}
	}
	count := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, db)
		_, err := testpkg.MeasuredTenantQuery1(t, counter.Context(ctx), db, counter, svc.List, true)
		require.NoError(t, err)
		return counter.Queries()
	}

	write(3)
	few := count()
	write(3)
	many := count()
	assert.Len(t, many, len(few), "the list does not grow with the letters")
	querybudgets.AssertQueryBudget(t, "modules.communication.school_parent_letters.list", many)
}

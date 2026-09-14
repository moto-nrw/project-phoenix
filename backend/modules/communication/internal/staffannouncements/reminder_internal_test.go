// Scheduled reminder tests (#3162). Pure in-memory: the repository, outbox and
// notifier are fakes; what the tests pin is the contract of the reminder —
// validation, the narrow post-publish edit, and that the due delivery reaches
// exactly the publication's audience over exactly the publication's channels,
// once.
package announcement

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reminderRepo adds the reminder repository slice to the publish-test fake.
type reminderRepo struct {
	fakeAnnouncementRepo
	due                []*usersModels.ParentAnnouncement
	claim              map[int64]bool
	claimCalls         []int64
	setReminderCalls   []setReminderCall
	setReminderApplied bool
	deliveryRecipients []*usersModels.AnnouncementDeliveryRecipient
}

type setReminderCall struct {
	id   int64
	at   *time.Time
	text *string
}

func (r *reminderRepo) ListDueReminders(_ context.Context, _, _ time.Time) ([]*usersModels.ParentAnnouncement, error) {
	return r.due, nil
}

func (r *reminderRepo) ClaimReminder(_ context.Context, id int64, _ time.Time) (bool, error) {
	r.claimCalls = append(r.claimCalls, id)
	claimed, ok := r.claim[id]
	return ok && claimed, nil
}

func (r *reminderRepo) SetReminder(_ context.Context, id int64, at *time.Time, text *string) (bool, error) {
	r.setReminderCalls = append(r.setReminderCalls, setReminderCall{id: id, at: at, text: text})
	return r.setReminderApplied, nil
}

func (r *reminderRepo) ResolveDeliveryRecipients(_ context.Context, _, _ int64) ([]*usersModels.AnnouncementDeliveryRecipient, error) {
	return r.deliveryRecipients, nil
}

type reminderHarness struct {
	repo     *reminderRepo
	outbox   *fakeOutbox
	notifier *fakeNotifier
	settings *fakeSettings
	svc      Service
}

func newReminderHarness(repo *reminderRepo) *reminderHarness {
	h := &reminderHarness{
		repo:     repo,
		outbox:   &fakeOutbox{},
		notifier: &fakeNotifier{},
		settings: &fakeSettings{enabled: true},
	}
	h.svc = NewService(ServiceConfig{
		Repo:       repo,
		Settings:   h.settings,
		Notifier:   h.notifier,
		Outbox:     h.outbox,
		ParentsURL: "https://parents.example.test",
		Logger:     slog.Default(),
	})
	h.svc.SetAttachmentPurger(&stubPurger{})
	return h
}

// publishedWithReminder is a live announcement whose reminder fell due.
func publishedWithReminder(sendEmail bool) *usersModels.ParentAnnouncement {
	a := draftAnnouncement(sendEmail)
	published := time.Now().Add(-14 * 24 * time.Hour)
	reminderAt := time.Now().Add(-time.Minute)
	text := "Morgen endet die Betreuung um 13:00 Uhr."
	a.PublishedAt = &published
	a.ReminderAt = &reminderAt
	a.ReminderText = &text
	return a
}

func strPtr(s string) *string { return &s }

func TestNormalizeReminder_Rules(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	future := now.Add(48 * time.Hour)
	past := now.Add(-time.Hour)
	expiry := now.Add(24 * time.Hour)

	for _, tc := range []struct {
		name    string
		in      Input
		wantErr bool
		check   func(t *testing.T, in Input)
	}{
		{name: "no reminder is fine", in: Input{}},
		{name: "text without a moment is rejected", in: Input{ReminderText: strPtr("Hallo")}, wantErr: true},
		{name: "blank text folds to nil", in: Input{ReminderAt: &future, ReminderText: strPtr("   ")},
			check: func(t *testing.T, in Input) { assert.Nil(t, in.ReminderText) }},
		{name: "text is trimmed", in: Input{ReminderAt: &future, ReminderText: strPtr("  Kurz  ")},
			check: func(t *testing.T, in Input) { assert.Equal(t, "Kurz", *in.ReminderText) }},
		{name: "overlong text is rejected", in: Input{ReminderAt: &future, ReminderText: strPtr(strings.Repeat("x", maxReminderTextLen+1))}, wantErr: true},
		{name: "a moment in the past is rejected", in: Input{ReminderAt: &past}, wantErr: true},
		{name: "a moment equal to now is rejected", in: Input{ReminderAt: &now}, wantErr: true},
		{name: "a moment after the expiry is rejected", in: Input{ReminderAt: &future, ExpiresAt: &expiry}, wantErr: true},
		{name: "a moment before the expiry passes", in: Input{ReminderAt: &future, ExpiresAt: ptrTime(future.Add(time.Hour))}},
		{name: "a poll never carries a scheduled reminder", in: Input{ReminderAt: &future, ResponseType: usersModels.ParentAnnouncementResponseSingleChoice}, wantErr: true},
		{name: "a letter may carry one", in: Input{ReminderAt: &future, DeliveryMode: usersModels.ParentAnnouncementDeliveryLetter}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := tc.in
			err := normalizeReminder(&in, now)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrValidation)
				return
			}
			require.NoError(t, err)
			if tc.check != nil {
				tc.check(t, in)
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestPublish_RejectsReminderAlreadyInThePast(t *testing.T) {
	t.Parallel()

	draft := draftAnnouncement(false)
	past := time.Now().Add(-time.Hour)
	draft.ReminderAt = &past
	repo := &reminderRepo{fakeAnnouncementRepo: fakeAnnouncementRepo{announcement: draft}}
	h := newReminderHarness(repo)

	_, err := h.svc.Publish(context.Background(), draft.ID)
	require.ErrorIs(t, err, ErrValidation)
	assert.Equal(t, 0, repo.publishCalls, "a draft with a dead reminder must not go live")
}

func TestUpdateReminder_RefusesOnceSent(t *testing.T) {
	t.Parallel()

	a := publishedWithReminder(false)
	sent := time.Now()
	a.ReminderSentAt = &sent
	repo := &reminderRepo{fakeAnnouncementRepo: fakeAnnouncementRepo{announcement: a}, setReminderApplied: true}
	h := newReminderHarness(repo)

	future := time.Now().Add(time.Hour)
	_, err := h.svc.UpdateReminder(context.Background(), a.ID, ReminderInput{ReminderAt: &future})
	require.ErrorIs(t, err, ErrReminderAlreadySent)
	assert.Empty(t, repo.setReminderCalls)
}

func TestUpdateReminder_RefusesSystemAnnouncement(t *testing.T) {
	t.Parallel()

	a := publishedWithReminder(false)
	a.ReminderSentAt = nil
	kind := usersModels.ParentAnnouncementSystemKindCareCancellation
	a.SystemKind = &kind
	repo := &reminderRepo{fakeAnnouncementRepo: fakeAnnouncementRepo{announcement: a}, setReminderApplied: true}
	h := newReminderHarness(repo)

	_, err := h.svc.UpdateReminder(context.Background(), a.ID, ReminderInput{})
	require.ErrorIs(t, err, ErrSystemAnnouncementImmutable)
}

func TestUpdateReminder_ValidatesMomentAgainstExpiry(t *testing.T) {
	t.Parallel()

	a := publishedWithReminder(false)
	expiry := time.Now().Add(24 * time.Hour)
	a.ExpiresAt = &expiry
	repo := &reminderRepo{fakeAnnouncementRepo: fakeAnnouncementRepo{announcement: a}, setReminderApplied: true}
	h := newReminderHarness(repo)

	late := expiry.Add(time.Hour)
	_, err := h.svc.UpdateReminder(context.Background(), a.ID, ReminderInput{ReminderAt: &late})
	require.ErrorIs(t, err, ErrValidation)

	past := time.Now().Add(-time.Hour)
	_, err = h.svc.UpdateReminder(context.Background(), a.ID, ReminderInput{ReminderAt: &past})
	require.ErrorIs(t, err, ErrValidation)
	assert.Empty(t, repo.setReminderCalls)
}

func TestUpdateReminder_RefusesPoll(t *testing.T) {
	t.Parallel()

	a := publishedWithReminder(false)
	a.ReminderAt = nil
	a.ReminderText = nil
	a.ResponseType = usersModels.ParentAnnouncementResponseSingleChoice
	repo := &reminderRepo{fakeAnnouncementRepo: fakeAnnouncementRepo{announcement: a}, setReminderApplied: true}
	h := newReminderHarness(repo)

	future := time.Now().Add(time.Hour)
	_, err := h.svc.UpdateReminder(context.Background(), a.ID, ReminderInput{ReminderAt: &future})
	require.ErrorIs(t, err, ErrValidation)
}

func TestUpdateReminder_MovesRewordsAndRemoves(t *testing.T) {
	t.Parallel()

	a := publishedWithReminder(false)
	repo := &reminderRepo{fakeAnnouncementRepo: fakeAnnouncementRepo{announcement: a}, setReminderApplied: true}
	h := newReminderHarness(repo)

	future := time.Now().Add(72 * time.Hour)
	got, err := h.svc.UpdateReminder(context.Background(), a.ID, ReminderInput{ReminderAt: &future, ReminderText: strPtr("  Neuer Text  ")})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, repo.setReminderCalls, 1)
	assert.Equal(t, future, *repo.setReminderCalls[0].at)
	assert.Equal(t, "Neuer Text", *repo.setReminderCalls[0].text)
	assert.Equal(t, 0, repo.updateCalls, "the reminder edit must not touch the immutable content columns")

	// Removing: nil moment, and any text sent along is dropped with it.
	_, err = h.svc.UpdateReminder(context.Background(), a.ID, ReminderInput{ReminderText: strPtr("bleibt nicht")})
	require.NoError(t, err)
	require.Len(t, repo.setReminderCalls, 2)
	assert.Nil(t, repo.setReminderCalls[1].at)
	assert.Nil(t, repo.setReminderCalls[1].text)
}

func TestUpdateReminder_LostRaceAgainstTheTickIsAConflict(t *testing.T) {
	t.Parallel()

	a := publishedWithReminder(false)
	repo := &reminderRepo{fakeAnnouncementRepo: fakeAnnouncementRepo{announcement: a}, setReminderApplied: false}
	h := newReminderHarness(repo)

	future := time.Now().Add(time.Hour)
	_, err := h.svc.UpdateReminder(context.Background(), a.ID, ReminderInput{ReminderAt: &future})
	require.ErrorIs(t, err, ErrReminderAlreadySent)
}

func TestSendDueReminders_ReachesTheWholeAudienceOnceOverBothChannels(t *testing.T) {
	t.Parallel()

	a := publishedWithReminder(true)
	acknowledged := time.Now().Add(-24 * time.Hour)
	repo := &reminderRepo{
		fakeAnnouncementRepo: fakeAnnouncementRepo{
			announcement: a,
			recipients: []*usersModels.AnnouncementRecipient{
				{AccountID: 101, Email: "Anna@example.test", FirstName: "Anna"},
				{AccountID: 102, Email: "ben@example.test", FirstName: "Ben"},
			},
			// One guardian already read AND confirmed: the reminder goes to them
			// regardless, that is the whole point of the feature.
			audience: []*usersModels.AnnouncementRecipientStatus{
				{AccountID: 101},
				{AccountID: 102, ReadAt: &acknowledged, AcknowledgedAt: &acknowledged},
			},
		},
		due:   []*usersModels.ParentAnnouncement{a},
		claim: map[int64]bool{a.ID: true},
	}
	h := newReminderHarness(repo)

	sent, err := h.svc.SendDueReminders(context.Background(), time.Now().Add(-time.Hour), time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, sent)
	assert.Equal(t, []int64{a.ID}, repo.claimCalls)

	require.Len(t, h.notifier.events, 1, "one push intent for the whole (single-locale) audience")
	event := h.notifier.events[0]
	assert.ElementsMatch(t, []int64{101, 102}, event.Audience.GuardianAccountIDs,
		"read and acknowledged guardians are reminded too")
	assert.Equal(t, relatedEntityTypeReminder, event.RelatedType)
	assert.Equal(t, "Erinnerung: Elternmitteilung", event.Title, "the push is recognisable as a reminder")
	assert.NotContains(t, event.Body, a.Body, "the push carries no announcement content")
	assert.Contains(t, event.IdempotencyKey, "parent-announcement-reminder:")

	require.Len(t, h.outbox.requests, 2, "one mail per address of the portal audience")
	for _, req := range h.outbox.requests {
		assert.Equal(t, relatedEntityTypeReminder, req.RelatedEntityType)
		assert.NotEmpty(t, req.IdempotencyKey, "a retried tick must not queue the mail twice")
		assert.Equal(t, "Erinnerung: "+a.Title, req.Payload[emailPayloadTitle])
		assert.Equal(t, reminderEmailKicker, req.Payload[emailPayloadKicker])
		assert.NotContains(t, req.Payload, emailPayloadBody, "a Mitteilung mail never carries the text")
	}
}

func TestSendDueReminders_WithoutEmailOptInSendsPushOnly(t *testing.T) {
	t.Parallel()

	a := publishedWithReminder(false)
	repo := &reminderRepo{
		fakeAnnouncementRepo: fakeAnnouncementRepo{
			announcement: a,
			recipients:   []*usersModels.AnnouncementRecipient{{AccountID: 101, Email: "anna@example.test"}},
		},
		due:   []*usersModels.ParentAnnouncement{a},
		claim: map[int64]bool{a.ID: true},
	}
	h := newReminderHarness(repo)

	sent, err := h.svc.SendDueReminders(context.Background(), time.Now().Add(-time.Hour), time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, sent)
	assert.Len(t, h.notifier.events, 1)
	assert.Empty(t, h.outbox.requests, "e-mail only when the announcement itself opted in")
}

func TestSendDueReminders_LostClaimSendsNothing(t *testing.T) {
	t.Parallel()

	a := publishedWithReminder(true)
	repo := &reminderRepo{
		fakeAnnouncementRepo: fakeAnnouncementRepo{
			announcement: a,
			recipients:   []*usersModels.AnnouncementRecipient{{AccountID: 101, Email: "anna@example.test"}},
		},
		due:   []*usersModels.ParentAnnouncement{a},
		claim: map[int64]bool{a.ID: false},
	}
	h := newReminderHarness(repo)

	sent, err := h.svc.SendDueReminders(context.Background(), time.Now().Add(-time.Hour), time.Now())
	require.NoError(t, err)
	assert.Equal(t, 0, sent)
	assert.Empty(t, h.notifier.events, "an overlapping tick already sent this reminder")
	assert.Empty(t, h.outbox.requests)
}

func TestSendDueReminders_NewsDisabledSendsNothing(t *testing.T) {
	t.Parallel()

	a := publishedWithReminder(true)
	repo := &reminderRepo{
		fakeAnnouncementRepo: fakeAnnouncementRepo{announcement: a},
		due:                  []*usersModels.ParentAnnouncement{a},
		claim:                map[int64]bool{a.ID: true},
	}
	h := newReminderHarness(repo)
	h.settings.enabled = false

	sent, err := h.svc.SendDueReminders(context.Background(), time.Now().Add(-time.Hour), time.Now())
	require.NoError(t, err)
	assert.Equal(t, 0, sent)
	assert.Empty(t, repo.claimCalls, "nothing is claimed while the school has the feature off")
}

func TestSendDueReminders_LetterCarriesTheReminderTextToItsWideAudience(t *testing.T) {
	t.Parallel()

	a := publishedWithReminder(true)
	a.DeliveryMode = usersModels.ParentAnnouncementDeliveryLetter
	a.EmailAudience = usersModels.EmailAudienceAllContacts
	a.RequiresAcknowledgement = true
	portalAccount := int64(101)
	repo := &reminderRepo{
		fakeAnnouncementRepo: fakeAnnouncementRepo{
			announcement: a,
			audience:     []*usersModels.AnnouncementRecipientStatus{{AccountID: portalAccount}},
		},
		due:   []*usersModels.ParentAnnouncement{a},
		claim: map[int64]bool{a.ID: true},
		deliveryRecipients: []*usersModels.AnnouncementDeliveryRecipient{
			{GuardianProfileID: 1, AccountID: &portalAccount, Email: "anna@example.test", HasPortalAccess: true},
			// No portal access, but the letter chose "alle Bezugspersonen":
			// they got the letter, so they get the reminder.
			{GuardianProfileID: 2, Email: "oma@example.test", HasPortalAccess: false},
			// No address: nothing to send, and nothing to fail on.
			{GuardianProfileID: 3, Email: "", HasPortalAccess: false},
		},
	}
	h := newReminderHarness(repo)

	sent, err := h.svc.SendDueReminders(context.Background(), time.Now().Add(-time.Hour), time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, sent)

	require.Len(t, h.outbox.requests, 2, "the reminder mails exactly the letter's audience")
	addresses := make([]string, 0, 2)
	for _, req := range h.outbox.requests {
		addresses = append(addresses, req.Payload[emailPayloadRecipient].(string))
		assert.Equal(t, letterReminderEmailKicker, req.Payload[emailPayloadKicker])
		assert.Equal(t, *a.ReminderText, req.Payload[emailPayloadBody], "a letter reminder carries the reminder wording")
		assert.Equal(t, false, req.Payload[emailPayloadAckRequired], "the reminder asks for no second confirmation")
		assert.Equal(t, relatedEntityTypeReminder, req.RelatedEntityType)
	}
	assert.ElementsMatch(t, []string{"anna@example.test", "oma@example.test"}, addresses)
}

func TestSendDueReminders_LetterWithoutOwnTextRepeatsTheBody(t *testing.T) {
	t.Parallel()

	a := publishedWithReminder(true)
	a.ReminderText = nil
	a.DeliveryMode = usersModels.ParentAnnouncementDeliveryLetter
	a.RequiresAcknowledgement = true
	portalAccount := int64(101)
	repo := &reminderRepo{
		fakeAnnouncementRepo: fakeAnnouncementRepo{
			announcement: a,
			audience:     []*usersModels.AnnouncementRecipientStatus{{AccountID: portalAccount}},
		},
		due:   []*usersModels.ParentAnnouncement{a},
		claim: map[int64]bool{a.ID: true},
		deliveryRecipients: []*usersModels.AnnouncementDeliveryRecipient{
			{GuardianProfileID: 1, AccountID: &portalAccount, Email: "anna@example.test", HasPortalAccess: true},
			// Portal-only letter: an address without portal access is NOT mailed.
			{GuardianProfileID: 2, Email: "oma@example.test", HasPortalAccess: false},
		},
	}
	h := newReminderHarness(repo)

	_, err := h.svc.SendDueReminders(context.Background(), time.Now().Add(-time.Hour), time.Now())
	require.NoError(t, err)
	require.Len(t, h.outbox.requests, 1)
	assert.Equal(t, a.Body, h.outbox.requests[0].Payload[emailPayloadBody])
}

func TestSendDueReminders_DeliveryFailureRollsTheTickBack(t *testing.T) {
	t.Parallel()

	a := publishedWithReminder(true)
	repo := &reminderRepo{
		fakeAnnouncementRepo: fakeAnnouncementRepo{
			announcement: a,
			recipients:   []*usersModels.AnnouncementRecipient{{AccountID: 101, Email: "anna@example.test"}},
		},
		due:   []*usersModels.ParentAnnouncement{a},
		claim: map[int64]bool{a.ID: true},
	}
	h := newReminderHarness(repo)
	h.outbox.enqueueErr = errors.New("outbox unavailable")

	_, err := h.svc.SendDueReminders(context.Background(), time.Now().Add(-time.Hour), time.Now())
	require.Error(t, err, "the caller's tenant transaction must roll back the claim with the failed delivery")
}

func TestReminderPushShape_IsKeyedOnTheReminderMoment(t *testing.T) {
	t.Parallel()

	a := publishedWithReminder(false)
	shape := reminderPushShape(a)
	assert.Equal(t, notifications.ParentAnnouncementReminder, shape.copyKind)
	assert.Equal(t, parentAnnouncementNotificationType, shape.notificationType, "consent is the announcement consent")
	assert.Equal(t, relatedEntityTypeReminder, shape.relatedType)

	moved := *a
	later := a.ReminderAt.Add(time.Hour)
	moved.ReminderAt = &later
	assert.NotEqual(t, shape.idempotencyKey, reminderPushShape(&moved).idempotencyKey)
}

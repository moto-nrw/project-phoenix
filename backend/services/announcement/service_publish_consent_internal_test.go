package announcement

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/notifications"
)

// fakePreferences is a stand-in for the consent service. optedIn is returned
// verbatim, so a test can express "these people agreed" directly.
type fakePreferences struct {
	notifications.PreferenceService
	optedIn []int64
	err     error
	// gotType records which notification type was asked about, so a test can
	// prove the producer filters on its own type rather than someone else's.
	gotType string
	asked   []int64

	// optedOut lists the accounts that explicitly declined. Everyone else stays
	// in, which is the whole point of the not-opted-out rule.
	optedOut      []int64
	optOutErr     error
	optOutGotType string
	askedOptOut   []int64
}

func (f *fakePreferences) FilterOptedIn(_ context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	f.gotType = notificationType
	f.asked = append([]int64(nil), accountIDs...)
	return f.optedIn, f.err
}

func (f *fakePreferences) FilterNotOptedOut(_ context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	f.optOutGotType = notificationType
	f.askedOptOut = append([]int64(nil), accountIDs...)
	if f.optOutErr != nil {
		return nil, f.optOutErr
	}
	declined := make(map[int64]struct{}, len(f.optedOut))
	for _, accountID := range f.optedOut {
		declined[accountID] = struct{}{}
	}
	remaining := make([]int64, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		if _, ok := declined[accountID]; ok {
			continue
		}
		remaining = append(remaining, accountID)
	}
	return remaining, nil
}

func consentTestService(repo *fakeAnnouncementRepo, notifier *fakeNotifier, prefs *fakePreferences) Service {
	return NewService(ServiceConfig{
		Repo:        repo,
		Settings:    &fakeSettings{enabled: true},
		Notifier:    notifier,
		Preferences: prefs,
		ParentsURL:  "https://parents.example.test",
		Logger:      slog.Default(),
	})
}

func consentTestRepo() *fakeAnnouncementRepo {
	return &fakeAnnouncementRepo{
		announcement: draftAnnouncement(false),
		audience: []*usersModels.AnnouncementRecipientStatus{
			{AccountID: 101},
			{AccountID: 202},
		},
	}
}

// TestPublishNotifiesOnlyGuardiansWhoAgreed pins the consent gate on the
// guardian push. The audience of an announcement says who it is FOR; consent
// says who agreed to be pushed about it. The second must narrow the first.
func TestPublishNotifiesOnlyGuardiansWhoAgreed(t *testing.T) {
	t.Parallel()

	repo := consentTestRepo()
	notifier := &fakeNotifier{}
	prefs := &fakePreferences{optedIn: []int64{202}}

	if _, err := consentTestService(repo, notifier, prefs).Publish(context.Background(), repo.announcement.ID); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	if len(notifier.events) != 1 {
		t.Fatalf("expected one guardian event, got %d", len(notifier.events))
	}
	if !slices.Equal(notifier.events[0].Audience.GuardianAccountIDs, []int64{202}) {
		t.Fatalf("only the guardian who agreed may be addressed, got %v",
			notifier.events[0].Audience.GuardianAccountIDs)
	}
	if prefs.gotType != notifications.TypeParentAnnouncement {
		t.Fatalf("the producer must filter on its own type, asked for %q", prefs.gotType)
	}
}

// TestPublishNotifiesNobodyWithoutConsent is the case that makes the default
// flip safe: the announcement still publishes, it simply pushes to nobody.
func TestPublishNotifiesNobodyWithoutConsent(t *testing.T) {
	t.Parallel()

	repo := consentTestRepo()
	notifier := &fakeNotifier{}

	published, err := consentTestService(repo, notifier, &fakePreferences{}).
		Publish(context.Background(), repo.announcement.ID)
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if published == nil {
		t.Fatal("the announcement must still be published")
	}
	if len(notifier.events) != 0 {
		t.Fatalf("nobody agreed, so nothing may be pushed, got %d events", len(notifier.events))
	}
}

// E-mail honours an explicit "no" and nothing else: the guardian who declined
// the type is dropped, the one who never decided still receives the mail. Push
// consent (FilterOptedIn) must not decide this, since most families have no row at
// all, so an opt-in rule here would silence the e-mail for nearly everybody.
func TestPublishEmailsEveryGuardianWhoDidNotOptOut(t *testing.T) {
	t.Parallel()

	repo := consentTestRepo()
	repo.announcement = draftAnnouncement(true)
	repo.recipients = []*usersModels.AnnouncementRecipient{
		{AccountID: 101, Email: "one@example.test"},
		{AccountID: 202, Email: "two@example.test"},
	}
	notifier := &fakeNotifier{}
	outbox := &fakeOutbox{}
	// 101 said no; 202 never touched the switch and is not opted in for push.
	prefs := &fakePreferences{optedOut: []int64{101}}
	svc := NewService(ServiceConfig{
		Repo:        repo,
		Settings:    &fakeSettings{enabled: true},
		Notifier:    notifier,
		Preferences: prefs,
		Outbox:      outbox,
		ParentsURL:  "https://parents.example.test",
		Logger:      slog.Default(),
	})

	if _, err := svc.Publish(context.Background(), repo.announcement.ID); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	if len(outbox.requests) != 1 {
		t.Fatalf("expected one e-mail for the guardian who did not decline, got %d", len(outbox.requests))
	}
	if got := outbox.requests[0].Payload[emailPayloadRecipient]; got != "two@example.test" {
		t.Fatalf("expected the address of the guardian who did not decline, got %v", got)
	}
	if !slices.Equal(prefs.askedOptOut, []int64{101, 202}) {
		t.Fatalf("expected the full e-mail audience to be checked for opt-outs, got %v", prefs.askedOptOut)
	}
	if prefs.optOutGotType != notifications.TypeParentAnnouncement {
		t.Fatalf("the e-mail path must filter on its own type, asked for %q", prefs.optOutGotType)
	}
}

// The regression this rule exists for: parent push is brand new, so a normal
// school has no preference rows and no push subscriptions at all. That must not
// stop the announcement e-mail, which schools have relied on all along.
func TestPublishEmailsFamiliesWithoutAnyPreferenceRow(t *testing.T) {
	t.Parallel()

	repo := consentTestRepo()
	repo.announcement = draftAnnouncement(true)
	repo.recipients = []*usersModels.AnnouncementRecipient{
		{AccountID: 101, Email: "one@example.test"},
		{AccountID: 202, Email: "two@example.test"},
	}
	notifier := &fakeNotifier{}
	outbox := &fakeOutbox{}
	// Nobody opted in (no push) and nobody opted out (no stored decision).
	prefs := &fakePreferences{}
	svc := NewService(ServiceConfig{
		Repo:        repo,
		Settings:    &fakeSettings{enabled: true},
		Notifier:    notifier,
		Preferences: prefs,
		Outbox:      outbox,
		ParentsURL:  "https://parents.example.test",
		Logger:      slog.Default(),
	})

	if _, err := svc.Publish(context.Background(), repo.announcement.ID); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	if len(notifier.events) != 0 {
		t.Fatalf("nobody agreed to push, so nothing may be pushed, got %d events", len(notifier.events))
	}
	if len(outbox.requests) != 2 {
		t.Fatalf("a missing decision must not silence the e-mail, got %d queued messages", len(outbox.requests))
	}
}

// A broken opt-out read must not be interpreted as "everyone agreed" either:
// the publish fails so staff can retry instead of guessing the audience.
func TestPublishFailsOnEmailOptOutLookupError(t *testing.T) {
	t.Parallel()

	repo := consentTestRepo()
	repo.announcement = draftAnnouncement(true)
	repo.recipients = []*usersModels.AnnouncementRecipient{
		{AccountID: 202, Email: "two@example.test"},
	}
	outbox := &fakeOutbox{}
	prefs := &fakePreferences{optedIn: []int64{202}, optOutErr: errors.New("boom")}
	svc := NewService(ServiceConfig{
		Repo:        repo,
		Settings:    &fakeSettings{enabled: true},
		Notifier:    &fakeNotifier{},
		Preferences: prefs,
		Outbox:      outbox,
		ParentsURL:  "https://parents.example.test",
		Logger:      slog.Default(),
	})

	if _, err := svc.Publish(context.Background(), repo.announcement.ID); err == nil {
		t.Fatal("a failed opt-out lookup must surface, not silently e-mail everyone")
	}
	if len(outbox.requests) != 0 {
		t.Fatalf("no e-mail may be queued on a failed lookup, got %d", len(outbox.requests))
	}
}

func TestPublishDeduplicatesOptedInGuardiansByNormalizedEmail(t *testing.T) {
	t.Parallel()

	repo := consentTestRepo()
	repo.announcement = draftAnnouncement(true)
	repo.recipients = []*usersModels.AnnouncementRecipient{
		{AccountID: 101, Email: " Family@Example.Test "},
		{AccountID: 202, Email: "family@example.test"},
	}
	notifier := &fakeNotifier{}
	outbox := &fakeOutbox{}
	prefs := &fakePreferences{optedIn: []int64{101, 202}}
	svc := NewService(ServiceConfig{
		Repo:        repo,
		Settings:    &fakeSettings{enabled: true},
		Notifier:    notifier,
		Preferences: prefs,
		Outbox:      outbox,
		ParentsURL:  "https://parents.example.test",
		Logger:      slog.Default(),
	})

	if _, err := svc.Publish(context.Background(), repo.announcement.ID); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	if len(outbox.requests) != 1 {
		t.Fatalf("expected one e-mail per normalized address, got %d", len(outbox.requests))
	}
	if got := outbox.requests[0].Payload[emailPayloadRecipient]; got != "family@example.test" {
		t.Fatalf("expected the normalized address, got %v", got)
	}
	if !slices.Equal(prefs.asked, []int64{101, 202}) {
		t.Fatalf("expected consent to be evaluated per account before address deduplication, got %v", prefs.asked)
	}
}

// The tenant dispatch switch and the active window govern push and in-app
// hints, not e-mail: they exist so a phone does not ring at night, and an
// e-mail does not ring. A gated push must therefore leave both the publication
// and the e-mail untouched.
func TestPublishStillEmailsWhenPushIsGated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		notifierErr error
	}{
		{name: "dispatch disabled", notifierErr: notifications.ErrDisabled},
		{name: "outside notification window", notifierErr: notifications.ErrOutsideActiveWindow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := consentTestRepo()
			repo.announcement = draftAnnouncement(true)
			repo.recipients = []*usersModels.AnnouncementRecipient{
				{AccountID: 202, Email: "two@example.test"},
			}
			notifier := &fakeNotifier{err: tt.notifierErr}
			prefs := &fakePreferences{optedIn: []int64{202}}
			outbox := &fakeOutbox{}
			svc := NewService(ServiceConfig{
				Repo:        repo,
				Settings:    &fakeSettings{enabled: true},
				Notifier:    notifier,
				Preferences: prefs,
				Outbox:      outbox,
				ParentsURL:  "https://parents.example.test",
				Logger:      slog.Default(),
			})

			published, err := svc.Publish(context.Background(), repo.announcement.ID)
			if err != nil {
				t.Fatalf("notification gate must not roll back publication: %v", err)
			}
			if published == nil {
				t.Fatal("the announcement must still be published")
			}
			if len(outbox.requests) != 1 {
				t.Fatalf("the push gate must not suppress e-mail, got %d queued messages", len(outbox.requests))
			}
			if got := outbox.requests[0].Payload[emailPayloadRecipient]; got != "two@example.test" {
				t.Fatalf("expected the audience address, got %v", got)
			}
		})
	}
}

// TestPublishFailsOnConsentLookupError keeps a broken consent read from being
// read as "nobody agreed" — that would look identical to a working feature.
func TestPublishFailsOnConsentLookupError(t *testing.T) {
	t.Parallel()

	repo := consentTestRepo()
	notifier := &fakeNotifier{}
	prefs := &fakePreferences{err: errors.New("boom")}

	if _, err := consentTestService(repo, notifier, prefs).Publish(context.Background(), repo.announcement.ID); err == nil {
		t.Fatal("a failed consent lookup must surface, not silently notify nobody")
	}
	if len(notifier.events) != 0 {
		t.Fatalf("no event may be dispatched on a failed lookup, got %d", len(notifier.events))
	}
}

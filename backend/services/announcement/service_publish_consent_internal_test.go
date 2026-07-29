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
}

func (f *fakePreferences) FilterOptedIn(_ context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	f.gotType = notificationType
	f.asked = append([]int64(nil), accountIDs...)
	return f.optedIn, f.err
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

func TestPublishEmailsOnlyGuardiansWhoAgreed(t *testing.T) {
	repo := consentTestRepo()
	repo.announcement = draftAnnouncement(true)
	repo.recipients = []*usersModels.AnnouncementRecipient{
		{AccountID: 101, Email: "one@example.test"},
		{AccountID: 202, Email: "two@example.test"},
	}
	notifier := &fakeNotifier{}
	outbox := &fakeOutbox{}
	prefs := &fakePreferences{optedIn: []int64{202}}
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
		t.Fatalf("expected one opted-in e-mail, got %d", len(outbox.requests))
	}
	if got := outbox.requests[0].Payload[emailPayloadRecipient]; got != "two@example.test" {
		t.Fatalf("expected the opted-in guardian's address, got %v", got)
	}
	if !slices.Equal(prefs.asked, []int64{101, 202}) {
		t.Fatalf("expected the full e-mail audience to be filtered, got %v", prefs.asked)
	}
}

// Tenant notification gates suppress every delivery channel, not the
// announcement itself. Staff must still be able to publish while dispatch is
// disabled or the notification window is closed.
func TestPublishSuppressesEmailWhenNotificationsAreGated(t *testing.T) {
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
			if len(outbox.requests) != 0 {
				t.Fatalf("notification gate must suppress e-mail too, got %d queued messages", len(outbox.requests))
			}
		})
	}
}

// TestPublishFailsOnConsentLookupError keeps a broken consent read from being
// read as "nobody agreed" — that would look identical to a working feature.
func TestPublishFailsOnConsentLookupError(t *testing.T) {
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

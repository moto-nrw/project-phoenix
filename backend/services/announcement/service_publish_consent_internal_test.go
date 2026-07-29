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
}

func (f *fakePreferences) FilterOptedIn(_ context.Context, notificationType string, _ []int64) ([]int64, error) {
	f.gotType = notificationType
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

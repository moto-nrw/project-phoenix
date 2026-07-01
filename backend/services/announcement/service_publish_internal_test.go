package announcement

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
)

// fakeAnnouncementRepo embeds the repository interface and overrides only what
// the unit under test touches; calling anything else panics loudly (nil embed),
// which is exactly what we want in a pure unit test.
type fakeAnnouncementRepo struct {
	usersModels.ParentAnnouncementRepository
	announcement *usersModels.ParentAnnouncement
	recipients   []*usersModels.AnnouncementRecipient
	updateCalls  int
	publishCalls int
}

func (f *fakeAnnouncementRepo) FindByID(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) {
	return f.announcement, nil
}

func (f *fakeAnnouncementRepo) Update(_ context.Context, _ *usersModels.ParentAnnouncement) error {
	f.updateCalls++
	return nil
}

func (f *fakeAnnouncementRepo) SetPublished(_ context.Context, _ int64, publishedAt *time.Time) error {
	f.publishCalls++
	f.announcement.PublishedAt = publishedAt
	return nil
}

func (f *fakeAnnouncementRepo) ReplaceTargets(_ context.Context, _, _ int64, _ []*usersModels.ParentAnnouncementTarget) error {
	return nil
}

func (f *fakeAnnouncementRepo) ListTargets(_ context.Context, _ int64) ([]*usersModels.ParentAnnouncementTarget, error) {
	return nil, nil
}

func (f *fakeAnnouncementRepo) ResolveAudienceEmails(_ context.Context, _, _ int64) ([]*usersModels.AnnouncementRecipient, error) {
	return f.recipients, nil
}

func (f *fakeAnnouncementRepo) SchoolName(_ context.Context, _ int64) (string, error) {
	return "OGS Testschule", nil
}

// fakeSettings answers the parent-news feature flag; everything else panics.
type fakeSettings struct {
	configService.SettingsService
	enabled bool
}

func (f *fakeSettings) ResolveBool(_ context.Context, _ string) (bool, error) {
	return f.enabled, nil
}

// fakeOutbox captures enqueued e-mails.
type fakeOutbox struct {
	requests []platformService.EnqueueRequest
}

func (f *fakeOutbox) Enqueue(_ context.Context, req platformService.EnqueueRequest) (*platformModels.EmailOutbox, error) {
	f.requests = append(f.requests, req)
	return &platformModels.EmailOutbox{}, nil
}

func newTestService(repo *fakeAnnouncementRepo, outbox *fakeOutbox) Service {
	return NewService(ServiceConfig{
		Repo:       repo,
		Settings:   &fakeSettings{enabled: true},
		Outbox:     outbox,
		ParentsURL: "https://parents.example.test",
		Logger:     slog.Default(),
	})
}

func draftAnnouncement(sendEmail bool) *usersModels.ParentAnnouncement {
	a := &usersModels.ParentAnnouncement{
		Title:     "Sommerfest",
		Body:      "Wir feiern am Freitag.",
		Priority:  usersModels.ParentAnnouncementPriorityInfo,
		SendEmail: sendEmail,
		Active:    true,
	}
	a.ID = 7 // in-memory sentinel, not a DB row
	a.SetTenantID(11)
	return a
}

func validInput() Input {
	return Input{
		Title:    "Sommerfest",
		Body:     "Neuer Text",
		Priority: usersModels.ParentAnnouncementPriorityInfo,
		Targets:  []TargetInput{{TargetType: usersModels.AnnouncementTargetSchoolAll}},
	}
}

func TestUpdate_PublishedIsImmutable(t *testing.T) {
	published := draftAnnouncement(false)
	now := time.Now()
	published.PublishedAt = &now
	repo := &fakeAnnouncementRepo{announcement: published}
	svc := newTestService(repo, &fakeOutbox{})

	_, err := svc.Update(context.Background(), published.ID, validInput())
	if !errors.Is(err, ErrPublishedImmutable) {
		t.Fatalf("expected ErrPublishedImmutable, got %v", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("expected no repo update on a published announcement, got %d", repo.updateCalls)
	}
}

func TestUpdate_DraftStaysEditable(t *testing.T) {
	repo := &fakeAnnouncementRepo{announcement: draftAnnouncement(false)}
	svc := newTestService(repo, &fakeOutbox{})

	if _, err := svc.Update(context.Background(), repo.announcement.ID, validInput()); err != nil {
		t.Fatalf("expected draft update to pass, got %v", err)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("expected exactly one repo update, got %d", repo.updateCalls)
	}
}

func TestPublish_EnqueuesTitleOnlyEmails(t *testing.T) {
	repo := &fakeAnnouncementRepo{
		announcement: draftAnnouncement(true),
		recipients: []*usersModels.AnnouncementRecipient{
			{Email: "a@example.test", FirstName: "Anna", LastName: "A"},
			{Email: "b@example.test", FirstName: "Ben", LastName: "B"},
		},
	}
	outbox := &fakeOutbox{}
	svc := newTestService(repo, outbox)

	if _, err := svc.Publish(context.Background(), repo.announcement.ID); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if repo.publishCalls != 1 {
		t.Fatalf("expected one SetPublished call, got %d", repo.publishCalls)
	}
	if len(outbox.requests) != 2 {
		t.Fatalf("expected one e-mail per recipient, got %d", len(outbox.requests))
	}
	for _, req := range outbox.requests {
		if req.Kind != platformModels.EmailKindParentAnnouncement {
			t.Fatalf("unexpected outbox kind %q", req.Kind)
		}
		if _, hasBody := req.Payload["body"]; hasBody {
			t.Fatalf("e-mail payload must not carry the announcement body (title + portal link only)")
		}
		if req.Payload[emailPayloadTitle] != "Sommerfest" {
			t.Fatalf("expected title in payload, got %v", req.Payload[emailPayloadTitle])
		}
		if req.Payload[emailPayloadPortalURL] != "https://parents.example.test" {
			t.Fatalf("expected portal url in payload, got %v", req.Payload[emailPayloadPortalURL])
		}
	}
}

func TestPublish_NoEmailWithoutOptIn(t *testing.T) {
	repo := &fakeAnnouncementRepo{
		announcement: draftAnnouncement(false),
		recipients: []*usersModels.AnnouncementRecipient{
			{Email: "a@example.test", FirstName: "Anna", LastName: "A"},
		},
	}
	outbox := &fakeOutbox{}
	svc := newTestService(repo, outbox)

	if _, err := svc.Publish(context.Background(), repo.announcement.ID); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if len(outbox.requests) != 0 {
		t.Fatalf("expected no e-mails without the per-announcement opt-in, got %d", len(outbox.requests))
	}
}

func TestPublish_RepublishDoesNotReEnqueue(t *testing.T) {
	repo := &fakeAnnouncementRepo{
		announcement: draftAnnouncement(true),
		recipients: []*usersModels.AnnouncementRecipient{
			{Email: "a@example.test", FirstName: "Anna", LastName: "A"},
		},
	}
	outbox := &fakeOutbox{}
	svc := newTestService(repo, outbox)

	if _, err := svc.Publish(context.Background(), repo.announcement.ID); err != nil {
		t.Fatalf("first publish failed: %v", err)
	}
	if _, err := svc.Publish(context.Background(), repo.announcement.ID); err != nil {
		t.Fatalf("second publish failed: %v", err)
	}
	if repo.publishCalls != 1 {
		t.Fatalf("expected publish to be idempotent, got %d SetPublished calls", repo.publishCalls)
	}
	if len(outbox.requests) != 1 {
		t.Fatalf("expected no duplicate e-mails on re-publish, got %d", len(outbox.requests))
	}
}

func TestAnnouncementRenderer_TitleAndLinkOnly(t *testing.T) {
	render := NewAnnouncementRenderer(EmailConfig{})
	msg, err := render(context.Background(), &platformModels.EmailOutbox{
		Kind: platformModels.EmailKindParentAnnouncement,
		Payload: map[string]any{
			emailPayloadRecipient:  "a@example.test",
			emailPayloadFirstName:  "Anna",
			emailPayloadLastName:   "A",
			emailPayloadTitle:      "Sommerfest",
			emailPayloadSchoolName: "OGS Testschule",
			emailPayloadPortalURL:  "https://parents.example.test",
		},
	})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if msg.Template != "announcement-published.html" {
		t.Fatalf("unexpected template %q", msg.Template)
	}
	if msg.Subject != "OGS Testschule: Sommerfest" {
		t.Fatalf("unexpected subject %q", msg.Subject)
	}
	content, ok := msg.Content.(map[string]any)
	if !ok {
		t.Fatalf("expected map content, got %T", msg.Content)
	}
	if _, hasBody := content["Body"]; hasBody {
		t.Fatalf("rendered content must not contain the announcement body")
	}
}

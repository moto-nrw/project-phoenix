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
	deleteCalls  int
	// editOnPublish, when set, replaces the stored row the instant PublishIfDraft
	// runs — simulating a concurrent edit that committed while PublishIfDraft was
	// blocked on the row lock. The pre-publish load still sees the original.
	editOnPublish *usersModels.ParentAnnouncement
}

func (f *fakeAnnouncementRepo) FindByID(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) {
	return f.announcement, nil
}

func (f *fakeAnnouncementRepo) Update(_ context.Context, _ *usersModels.ParentAnnouncement) error {
	f.updateCalls++
	return nil
}

func (f *fakeAnnouncementRepo) SetPublished(_ context.Context, _ int64, publishedAt *time.Time) error {
	f.announcement.PublishedAt = publishedAt
	return nil
}

func (f *fakeAnnouncementRepo) PublishIfDraft(_ context.Context, _ int64, publishedAt time.Time) (bool, error) {
	f.publishCalls++
	if f.editOnPublish != nil {
		f.announcement = f.editOnPublish
	}
	f.announcement.PublishedAt = &publishedAt
	return true, nil
}

func (f *fakeAnnouncementRepo) Delete(_ context.Context, _ int64) error {
	f.deleteCalls++
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
	logoURL string
}

func (f *fakeSettings) ResolveBool(_ context.Context, _ string) (bool, error) {
	return f.enabled, nil
}

func (f *fakeSettings) GetLoginImageURL(_ context.Context, _ int64) (string, error) {
	return f.logoURL, nil
}

// fakeOutbox captures enqueued e-mails.
type fakeOutbox struct {
	requests    []platformService.EnqueueRequest
	cancelCalls int
}

func (f *fakeOutbox) Enqueue(_ context.Context, req platformService.EnqueueRequest) (*platformModels.EmailOutbox, error) {
	f.requests = append(f.requests, req)
	return &platformModels.EmailOutbox{}, nil
}

func (f *fakeOutbox) CancelPendingByRelatedEntity(_ context.Context, _ string, _ int64, _ string) (int64, error) {
	f.cancelCalls++
	return 0, nil
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

func TestCreate_RejectsPastExpiry(t *testing.T) {
	repo := &fakeAnnouncementRepo{announcement: draftAnnouncement(false)}
	svc := newTestService(repo, &fakeOutbox{})

	in := validInput()
	past := time.Now().Add(-24 * time.Hour)
	in.ExpiresAt = &past

	_, err := svc.Create(context.Background(), 42, in)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for an expires_at in the past, got %v", err)
	}
}

func TestUnpublish_CancelsPendingEmails(t *testing.T) {
	published := draftAnnouncement(true)
	now := time.Now()
	published.PublishedAt = &now
	repo := &fakeAnnouncementRepo{announcement: published}
	outbox := &fakeOutbox{}
	svc := newTestService(repo, outbox)

	if _, err := svc.Unpublish(context.Background(), published.ID); err != nil {
		t.Fatalf("unpublish failed: %v", err)
	}
	if outbox.cancelCalls != 1 {
		t.Fatalf("expected unpublish to cancel pending e-mails once, got %d", outbox.cancelCalls)
	}
}

func TestDelete_CancelsPendingEmails(t *testing.T) {
	published := draftAnnouncement(true)
	now := time.Now()
	published.PublishedAt = &now
	repo := &fakeAnnouncementRepo{announcement: published}
	outbox := &fakeOutbox{}
	svc := newTestService(repo, outbox)

	if err := svc.Delete(context.Background(), published.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if outbox.cancelCalls != 1 {
		t.Fatalf("expected delete to cancel pending e-mails once, got %d", outbox.cancelCalls)
	}
	if repo.deleteCalls != 1 {
		t.Fatalf("expected exactly one repo delete, got %d", repo.deleteCalls)
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
		t.Fatalf("expected one PublishIfDraft call, got %d", repo.publishCalls)
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

// The announcement mail must carry the same header/footer branding as the
// enrollment mails: an absolute school-logo URL (uploaded image rewritten to the
// public read endpoint) and the absolute moto footer logo.
func TestPublish_EnqueuesBrandingLogos(t *testing.T) {
	repo := &fakeAnnouncementRepo{
		announcement: draftAnnouncement(true),
		recipients: []*usersModels.AnnouncementRecipient{
			{Email: "a@example.test", FirstName: "Anna", LastName: "A"},
		},
	}
	outbox := &fakeOutbox{}
	svc := NewService(ServiceConfig{
		Repo:       repo,
		Settings:   &fakeSettings{enabled: true, logoURL: "/uploads/login-images/2_abc.jpg"},
		Outbox:     outbox,
		ParentsURL: "https://parents.example.test",
		Logger:     slog.Default(),
	})

	if _, err := svc.Publish(context.Background(), repo.announcement.ID); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if len(outbox.requests) != 1 {
		t.Fatalf("expected one e-mail, got %d", len(outbox.requests))
	}
	payload := outbox.requests[0].Payload
	if got := payload[emailPayloadMotoLogoURL]; got != "https://parents.example.test/images/moto-logo-mit-schriftzug.png" {
		t.Fatalf("unexpected moto logo url: %v", got)
	}
	if got := payload[emailPayloadLogoURL]; got != "https://parents.example.test/api/public/login-image/2_abc.jpg" {
		t.Fatalf("unexpected school logo url: %v", got)
	}
}

// A school without a configured logo still gets the moto footer logo; the school
// logo degrades to "" so the template shows the plain fallback.
func TestPublish_BrandingWithoutSchoolLogo(t *testing.T) {
	repo := &fakeAnnouncementRepo{
		announcement: draftAnnouncement(true),
		recipients: []*usersModels.AnnouncementRecipient{
			{Email: "a@example.test", FirstName: "Anna", LastName: "A"},
		},
	}
	outbox := &fakeOutbox{}
	svc := newTestService(repo, outbox)

	if _, err := svc.Publish(context.Background(), repo.announcement.ID); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	payload := outbox.requests[0].Payload
	if got := payload[emailPayloadMotoLogoURL]; got != "https://parents.example.test/images/moto-logo-mit-schriftzug.png" {
		t.Fatalf("moto logo must always be present, got %v", got)
	}
	if got := payload[emailPayloadLogoURL]; got != "" {
		t.Fatalf("expected empty school logo without a configured image, got %v", got)
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
		t.Fatalf("expected publish to be idempotent, got %d PublishIfDraft calls", repo.publishCalls)
	}
	if len(outbox.requests) != 1 {
		t.Fatalf("expected no duplicate e-mails on re-publish, got %d", len(outbox.requests))
	}
}

// TestPublish_ConcurrentEditWins_UsesFreshRow proves the publish path decides
// from the post-flip row, not the pre-publish snapshot. A concurrent edit that
// committed while PublishIfDraft held the row lock toggled send_email off and
// changed the title; the winner must honour the fresh state (no e-mail here),
// never re-send with the stale opt-in / old title.
func TestPublish_ConcurrentEditWins_UsesFreshRow(t *testing.T) {
	edited := draftAnnouncement(false) // send_email flipped OFF by the concurrent edit
	edited.Title = "Sommerfest (verschoben)"
	repo := &fakeAnnouncementRepo{
		announcement:  draftAnnouncement(true), // pre-publish snapshot: opted INTO e-mail
		editOnPublish: edited,
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
		t.Fatalf("expected no e-mails: the concurrent edit turned send_email off, got %d", len(outbox.requests))
	}
}

// TestPublish_ConcurrentEditExpires_RollsBack proves that when a concurrent edit
// moves expires_at into the past during the flip, the publish fails (so the
// tenant tx rolls the flip back) instead of publishing an invisible, immutable,
// already-expired announcement.
func TestPublish_ConcurrentEditExpires_RollsBack(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	edited := draftAnnouncement(true)
	edited.ExpiresAt = &past // concurrent edit expired it
	repo := &fakeAnnouncementRepo{
		announcement:  draftAnnouncement(true), // pre-publish snapshot: no expiry, passes the pre-check
		editOnPublish: edited,
		recipients: []*usersModels.AnnouncementRecipient{
			{Email: "a@example.test", FirstName: "Anna", LastName: "A"},
		},
	}
	outbox := &fakeOutbox{}
	svc := newTestService(repo, outbox)

	if _, err := svc.Publish(context.Background(), repo.announcement.ID); err == nil {
		t.Fatal("expected publish to fail when the row expired concurrently during the flip")
	}
	if len(outbox.requests) != 0 {
		t.Fatalf("expected no e-mails for an expired-during-publish row, got %d", len(outbox.requests))
	}
}

// failingOutbox always fails Enqueue, standing in for a DB error on the outbox
// insert (which, inside the publish tenant tx, aborts the whole transaction).
type failingOutbox struct{}

func (failingOutbox) Enqueue(_ context.Context, _ platformService.EnqueueRequest) (*platformModels.EmailOutbox, error) {
	return nil, errors.New("outbox insert failed")
}

func (failingOutbox) CancelPendingByRelatedEntity(_ context.Context, _ string, _ int64, _ string) (int64, error) {
	return 0, nil
}

// A failed e-mail enqueue must fail the publish rather than being swallowed: the
// insert runs in the publish transaction, so a DB error aborts it and the commit
// would fail anyway. Publish must surface the error, not report false success.
func TestPublish_EmailEnqueueFailureIsFatal(t *testing.T) {
	repo := &fakeAnnouncementRepo{
		announcement: draftAnnouncement(true),
		recipients: []*usersModels.AnnouncementRecipient{
			{Email: "a@example.test", FirstName: "Anna", LastName: "A"},
		},
	}
	svc := NewService(ServiceConfig{
		Repo:       repo,
		Settings:   &fakeSettings{enabled: true},
		Outbox:     failingOutbox{},
		ParentsURL: "https://parents.example.test",
		Logger:     slog.Default(),
	})

	if _, err := svc.Publish(context.Background(), repo.announcement.ID); err == nil {
		t.Fatal("expected publish to fail when the e-mail enqueue fails")
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

// The renderer must forward the logo URLs to the template so the header renders
// the school logo and the footer renders the moto logo.
func TestAnnouncementRenderer_PassesBranding(t *testing.T) {
	render := NewAnnouncementRenderer(EmailConfig{})
	msg, err := render(context.Background(), &platformModels.EmailOutbox{
		Kind: platformModels.EmailKindParentAnnouncement,
		Payload: map[string]any{
			emailPayloadRecipient:   "a@example.test",
			emailPayloadTitle:       "Sommerfest",
			emailPayloadSchoolName:  "OGS Testschule",
			emailPayloadPortalURL:   "https://parents.example.test",
			emailPayloadLogoURL:     "https://parents.example.test/api/public/login-image/2_abc.jpg",
			emailPayloadMotoLogoURL: "https://parents.example.test/images/moto-logo-mit-schriftzug.png",
		},
	})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	content, ok := msg.Content.(map[string]any)
	if !ok {
		t.Fatalf("expected map content, got %T", msg.Content)
	}
	if content["LogoURL"] != "https://parents.example.test/api/public/login-image/2_abc.jpg" {
		t.Fatalf("expected school logo in content, got %v", content["LogoURL"])
	}
	if content["MotoLogoURL"] != "https://parents.example.test/images/moto-logo-mit-schriftzug.png" {
		t.Fatalf("expected moto logo in content, got %v", content["MotoLogoURL"])
	}
}

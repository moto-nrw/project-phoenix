package announcement

import (
	"context"
	"errors"
	"testing"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
)

// --- test doubles -----------------------------------------------------------

// mockRepo is a func-field mock of the ParentAnnouncementRepository. Only the
// methods a given test exercises need a func; the rest panic if called
// unexpectedly, which keeps each test honest about the repository calls it
// drives.
type mockRepo struct {
	createFn        func(ctx context.Context, a *usersModels.ParentAnnouncement) error
	updateFn        func(ctx context.Context, a *usersModels.ParentAnnouncement) error
	findByIDFn      func(ctx context.Context, id int64) (*usersModels.ParentAnnouncement, error)
	deleteFn        func(ctx context.Context, id int64) error
	listForTenantFn func(ctx context.Context, includeInactive bool) ([]*usersModels.ParentAnnouncement, error)
	setPublishedFn  func(ctx context.Context, id int64, publishedAt *time.Time) error
	publishIfDraft  func(ctx context.Context, id int64, publishedAt time.Time) (bool, error)
	replaceTargets  func(ctx context.Context, tenantID, announcementID int64, targets []*usersModels.ParentAnnouncementTarget) error
	listTargetsFn   func(ctx context.Context, announcementID int64) ([]*usersModels.ParentAnnouncementTarget, error)
	resolveEmailsFn func(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.AnnouncementRecipient, error)
	schoolNameFn    func(ctx context.Context, tenantID int64) (string, error)
	audienceFn      func(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.AnnouncementRecipientStatus, error)
	statsFn         func(ctx context.Context, tenantID, announcementID int64) (*usersModels.AnnouncementStats, error)
}

func (m *mockRepo) Create(ctx context.Context, a *usersModels.ParentAnnouncement) error {
	return m.createFn(ctx, a)
}
func (m *mockRepo) Update(ctx context.Context, a *usersModels.ParentAnnouncement) error {
	return m.updateFn(ctx, a)
}
func (m *mockRepo) FindByID(ctx context.Context, id int64) (*usersModels.ParentAnnouncement, error) {
	return m.findByIDFn(ctx, id)
}
func (m *mockRepo) Delete(ctx context.Context, id int64) error { return m.deleteFn(ctx, id) }
func (m *mockRepo) ListForTenant(ctx context.Context, includeInactive bool) ([]*usersModels.ParentAnnouncement, error) {
	return m.listForTenantFn(ctx, includeInactive)
}
func (m *mockRepo) SetPublished(ctx context.Context, id int64, publishedAt *time.Time) error {
	return m.setPublishedFn(ctx, id, publishedAt)
}
func (m *mockRepo) PublishIfDraft(ctx context.Context, id int64, publishedAt time.Time) (bool, error) {
	return m.publishIfDraft(ctx, id, publishedAt)
}
func (m *mockRepo) ReplaceTargets(ctx context.Context, tenantID, announcementID int64, targets []*usersModels.ParentAnnouncementTarget) error {
	return m.replaceTargets(ctx, tenantID, announcementID, targets)
}
func (m *mockRepo) ListTargets(ctx context.Context, announcementID int64) ([]*usersModels.ParentAnnouncementTarget, error) {
	return m.listTargetsFn(ctx, announcementID)
}
func (m *mockRepo) CountAudience(ctx context.Context, tenantID, announcementID int64) (int, error) {
	return 0, nil
}
func (m *mockRepo) AccountMatchesAnnouncement(ctx context.Context, tenantID, announcementID, accountID int64) (bool, error) {
	return false, nil
}
func (m *mockRepo) ResolveAudienceEmails(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.AnnouncementRecipient, error) {
	return m.resolveEmailsFn(ctx, tenantID, announcementID)
}
func (m *mockRepo) SchoolName(ctx context.Context, tenantID int64) (string, error) {
	return m.schoolNameFn(ctx, tenantID)
}
func (m *mockRepo) AudienceRecipients(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.AnnouncementRecipientStatus, error) {
	return m.audienceFn(ctx, tenantID, announcementID)
}
func (m *mockRepo) ListFeedForAccount(ctx context.Context, accountID int64, tenantIDs []int64) ([]*usersModels.AnnouncementFeedItem, error) {
	return nil, nil
}
func (m *mockRepo) CountUnreadForAccount(ctx context.Context, accountID int64, tenantIDs []int64) (int, error) {
	return 0, nil
}
func (m *mockRepo) MarkRead(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error) {
	return false, nil
}
func (m *mockRepo) MarkAcknowledged(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error) {
	return false, nil
}
func (m *mockRepo) Stats(ctx context.Context, tenantID, announcementID int64) (*usersModels.AnnouncementStats, error) {
	return m.statsFn(ctx, tenantID, announcementID)
}

// stubSettings satisfies configService.SettingsService via the embedded (nil)
// interface; only the two methods the service touches are implemented. Any
// other call panics — the service must not reach for settings it does not use.
type stubSettings struct {
	configService.SettingsService
	newsEnabled   bool
	newsErr       error
	loginImageURL string
	loginImageErr error
}

func (s stubSettings) ResolveBool(_ context.Context, _ string) (bool, error) {
	return s.newsEnabled, s.newsErr
}
func (s stubSettings) GetLoginImageURL(_ context.Context, _ int64) (string, error) {
	return s.loginImageURL, s.loginImageErr
}

type stubOutbox struct {
	enqueueFn func(ctx context.Context, req platformService.EnqueueRequest) (*platformModels.EmailOutbox, error)
	cancelFn  func(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error)
}

func (o *stubOutbox) Enqueue(ctx context.Context, req platformService.EnqueueRequest) (*platformModels.EmailOutbox, error) {
	return o.enqueueFn(ctx, req)
}
func (o *stubOutbox) CancelPendingByRelatedEntity(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error) {
	return o.cancelFn(ctx, relatedType, relatedID, reason)
}

const (
	testAnnID    = int64(555)
	testTenantID = int64(42)
)

func draft() *usersModels.ParentAnnouncement {
	a := &usersModels.ParentAnnouncement{
		Title:    "Sommerfest",
		Body:     "Wir feiern am Freitag.",
		Priority: usersModels.ParentAnnouncementPriorityInfo,
		Active:   true,
	}
	a.ID = testAnnID
	a.SetTenantID(testTenantID)
	return a
}

func published() *usersModels.ParentAnnouncement {
	a := draft()
	now := time.Now()
	a.PublishedAt = &now
	return a
}

func validInputM() Input {
	return Input{
		Title:    "Sommerfest",
		Body:     "Wir feiern am Freitag.",
		Priority: usersModels.ParentAnnouncementPriorityImportant,
		Targets:  []TargetInput{{TargetType: usersModels.AnnouncementTargetSchoolAll}},
	}
}

// --- List -------------------------------------------------------------------

func TestService_List(t *testing.T) {
	want := []*usersModels.ParentAnnouncement{draft()}
	repo := &mockRepo{listForTenantFn: func(_ context.Context, includeInactive bool) ([]*usersModels.ParentAnnouncement, error) {
		if !includeInactive {
			t.Fatalf("expected includeInactive=true")
		}
		return want, nil
	}}
	svc := NewService(ServiceConfig{Repo: repo})
	got, err := svc.List(context.Background(), true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("List returned %v", got)
	}
}

func TestService_List_RepoError(t *testing.T) {
	repo := &mockRepo{listForTenantFn: func(_ context.Context, _ bool) ([]*usersModels.ParentAnnouncement, error) {
		return nil, errors.New("db down")
	}}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.List(context.Background(), false); err == nil {
		t.Fatal("expected error")
	}
}

// --- Get --------------------------------------------------------------------

func TestService_Get(t *testing.T) {
	targets := []*usersModels.ParentAnnouncementTarget{{TargetType: usersModels.AnnouncementTargetSchoolAll}}
	repo := &mockRepo{
		findByIDFn:    func(_ context.Context, id int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil },
		listTargetsFn: func(_ context.Context, _ int64) ([]*usersModels.ParentAnnouncementTarget, error) { return targets, nil },
	}
	svc := NewService(ServiceConfig{Repo: repo})
	got, err := svc.Get(context.Background(), testAnnID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Targets) != 1 {
		t.Fatalf("expected targets attached, got %v", got.Targets)
	}
}

func TestService_Get_NotFound(t *testing.T) {
	repo := &mockRepo{findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return nil, nil }}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.Get(context.Background(), testAnnID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestService_Get_TargetsError(t *testing.T) {
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil },
		listTargetsFn: func(_ context.Context, _ int64) ([]*usersModels.ParentAnnouncementTarget, error) {
			return nil, errors.New("boom")
		},
	}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.Get(context.Background(), testAnnID); err == nil {
		t.Fatal("expected error")
	}
}

// --- Create -----------------------------------------------------------------

func TestService_Create(t *testing.T) {
	var replaced bool
	repo := &mockRepo{
		createFn: func(_ context.Context, a *usersModels.ParentAnnouncement) error {
			a.ID = testAnnID
			a.SetTenantID(testTenantID)
			return nil
		},
		replaceTargets: func(_ context.Context, tenantID, announcementID int64, targets []*usersModels.ParentAnnouncementTarget) error {
			replaced = true
			if tenantID != testTenantID {
				t.Fatalf("expected tenant %d, got %d", testTenantID, tenantID)
			}
			return nil
		},
	}
	svc := NewService(ServiceConfig{Repo: repo, Settings: stubSettings{newsEnabled: true}})
	got, err := svc.Create(context.Background(), 9, validInputM())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !replaced || got.CreatedBy != 9 || len(got.Targets) != 1 {
		t.Fatalf("unexpected create result: replaced=%v got=%+v", replaced, got)
	}
}

func TestService_Create_NewsDisabled(t *testing.T) {
	svc := NewService(ServiceConfig{Repo: &mockRepo{}, Settings: stubSettings{newsEnabled: false}})
	if _, err := svc.Create(context.Background(), 1, validInputM()); !errors.Is(err, ErrNewsDisabled) {
		t.Fatalf("expected ErrNewsDisabled, got %v", err)
	}
}

func TestService_Create_FlagError(t *testing.T) {
	svc := NewService(ServiceConfig{Repo: &mockRepo{}, Settings: stubSettings{newsErr: errors.New("settings down")}})
	if _, err := svc.Create(context.Background(), 1, validInputM()); err == nil {
		t.Fatal("expected error")
	}
}

func TestService_Create_InvalidInput(t *testing.T) {
	svc := NewService(ServiceConfig{Repo: &mockRepo{}, Settings: stubSettings{newsEnabled: true}})
	in := validInputM()
	in.Title = "   " // empty after trim
	if _, err := svc.Create(context.Background(), 1, in); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestService_Create_RepoError(t *testing.T) {
	repo := &mockRepo{createFn: func(_ context.Context, _ *usersModels.ParentAnnouncement) error { return errors.New("insert failed") }}
	svc := NewService(ServiceConfig{Repo: repo, Settings: stubSettings{newsEnabled: true}})
	if _, err := svc.Create(context.Background(), 1, validInputM()); err == nil {
		t.Fatal("expected error")
	}
}

func TestService_Create_TargetsError(t *testing.T) {
	repo := &mockRepo{
		createFn: func(_ context.Context, a *usersModels.ParentAnnouncement) error { a.ID = testAnnID; return nil },
		replaceTargets: func(_ context.Context, _, _ int64, _ []*usersModels.ParentAnnouncementTarget) error {
			return errors.New("targets failed")
		},
	}
	svc := NewService(ServiceConfig{Repo: repo, Settings: stubSettings{newsEnabled: true}})
	if _, err := svc.Create(context.Background(), 1, validInputM()); err == nil {
		t.Fatal("expected error")
	}
}

// --- Update -----------------------------------------------------------------

func TestService_Update(t *testing.T) {
	repo := &mockRepo{
		findByIDFn:     func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil },
		updateFn:       func(_ context.Context, _ *usersModels.ParentAnnouncement) error { return nil },
		replaceTargets: func(_ context.Context, _, _ int64, _ []*usersModels.ParentAnnouncementTarget) error { return nil },
	}
	svc := NewService(ServiceConfig{Repo: repo})
	got, err := svc.Update(context.Background(), testAnnID, validInputM())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Title != "Sommerfest" {
		t.Fatalf("unexpected update result: %+v", got)
	}
}

func TestService_Update_NotFound(t *testing.T) {
	repo := &mockRepo{findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return nil, nil }}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.Update(context.Background(), testAnnID, validInputM()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestService_Update_PublishedImmutable(t *testing.T) {
	repo := &mockRepo{findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return published(), nil }}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.Update(context.Background(), testAnnID, validInputM()); !errors.Is(err, ErrPublishedImmutable) {
		t.Fatalf("expected ErrPublishedImmutable, got %v", err)
	}
}

func TestService_Update_PublishedRace(t *testing.T) {
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil },
		updateFn: func(_ context.Context, _ *usersModels.ParentAnnouncement) error {
			return usersModels.ErrAnnouncementPublished
		},
	}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.Update(context.Background(), testAnnID, validInputM()); !errors.Is(err, ErrPublishedImmutable) {
		t.Fatalf("expected ErrPublishedImmutable on update race, got %v", err)
	}
}

func TestService_Update_TargetsPublishedRace(t *testing.T) {
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil },
		updateFn:   func(_ context.Context, _ *usersModels.ParentAnnouncement) error { return nil },
		replaceTargets: func(_ context.Context, _, _ int64, _ []*usersModels.ParentAnnouncementTarget) error {
			return usersModels.ErrAnnouncementPublished
		},
	}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.Update(context.Background(), testAnnID, validInputM()); !errors.Is(err, ErrPublishedImmutable) {
		t.Fatalf("expected ErrPublishedImmutable on targets race, got %v", err)
	}
}

// --- Delete -----------------------------------------------------------------

func TestService_Delete(t *testing.T) {
	var cancelled, deleted bool
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil },
		deleteFn:   func(_ context.Context, _ int64) error { deleted = true; return nil },
	}
	outbox := &stubOutbox{cancelFn: func(_ context.Context, _ string, _ int64, _ string) (int64, error) { cancelled = true; return 2, nil }}
	svc := NewService(ServiceConfig{Repo: repo, Outbox: outbox})
	if err := svc.Delete(context.Background(), testAnnID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !cancelled || !deleted {
		t.Fatalf("expected cancel+delete, got cancelled=%v deleted=%v", cancelled, deleted)
	}
}

func TestService_Delete_NotFound(t *testing.T) {
	repo := &mockRepo{findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return nil, nil }}
	svc := NewService(ServiceConfig{Repo: repo})
	if err := svc.Delete(context.Background(), testAnnID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestService_Delete_CancelError(t *testing.T) {
	repo := &mockRepo{findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil }}
	outbox := &stubOutbox{cancelFn: func(_ context.Context, _ string, _ int64, _ string) (int64, error) {
		return 0, errors.New("cancel failed")
	}}
	svc := NewService(ServiceConfig{Repo: repo, Outbox: outbox})
	if err := svc.Delete(context.Background(), testAnnID); err == nil {
		t.Fatal("expected cancel error to propagate")
	}
}

// --- Publish ----------------------------------------------------------------

func TestService_Publish_WithEmail(t *testing.T) {
	var enqueued int
	fresh := draft()
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) {
			f := *fresh
			f.SendEmail = true
			return &f, nil
		},
		publishIfDraft: func(_ context.Context, _ int64, _ time.Time) (bool, error) { return true, nil },
		listTargetsFn:  func(_ context.Context, _ int64) ([]*usersModels.ParentAnnouncementTarget, error) { return nil, nil },
		resolveEmailsFn: func(_ context.Context, _, _ int64) ([]*usersModels.AnnouncementRecipient, error) {
			return []*usersModels.AnnouncementRecipient{{Email: "a@b.de", FirstName: "Ada", LastName: "L"}}, nil
		},
		schoolNameFn: func(_ context.Context, _ int64) (string, error) { return "OGS Am Berg", nil },
	}
	outbox := &stubOutbox{enqueueFn: func(_ context.Context, req platformService.EnqueueRequest) (*platformModels.EmailOutbox, error) {
		enqueued++
		if req.Payload[emailPayloadTitle] != "Sommerfest" {
			t.Fatalf("expected title in payload, got %v", req.Payload[emailPayloadTitle])
		}
		if _, ok := req.Payload["body"]; ok {
			t.Fatal("e-mail payload must not carry the body")
		}
		return &platformModels.EmailOutbox{}, nil
	}}
	svc := NewService(ServiceConfig{
		Repo:       repo,
		Settings:   stubSettings{newsEnabled: true, loginImageURL: "logo.png"},
		Outbox:     outbox,
		ParentsURL: "https://parents.example",
	})
	if _, err := svc.Publish(context.Background(), testAnnID); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if enqueued != 1 {
		t.Fatalf("expected 1 e-mail enqueued, got %d", enqueued)
	}
}

func TestService_Publish_AlreadyPublished_NoEmail(t *testing.T) {
	repo := &mockRepo{
		findByIDFn:    func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return published(), nil },
		listTargetsFn: func(_ context.Context, _ int64) ([]*usersModels.ParentAnnouncementTarget, error) { return nil, nil },
	}
	// PublishIfDraft must not be called for an already-published row.
	svc := NewService(ServiceConfig{Repo: repo, Settings: stubSettings{newsEnabled: true}})
	if _, err := svc.Publish(context.Background(), testAnnID); err != nil {
		t.Fatalf("Publish already-published: %v", err)
	}
}

func TestService_Publish_LostRace_NoEmail(t *testing.T) {
	repo := &mockRepo{
		findByIDFn:     func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil },
		publishIfDraft: func(_ context.Context, _ int64, _ time.Time) (bool, error) { return false, nil },
		listTargetsFn:  func(_ context.Context, _ int64) ([]*usersModels.ParentAnnouncementTarget, error) { return nil, nil },
	}
	svc := NewService(ServiceConfig{Repo: repo, Settings: stubSettings{newsEnabled: true}})
	if _, err := svc.Publish(context.Background(), testAnnID); err != nil {
		t.Fatalf("Publish lost race: %v", err)
	}
}

func TestService_Publish_NewsDisabled(t *testing.T) {
	repo := &mockRepo{findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil }}
	svc := NewService(ServiceConfig{Repo: repo, Settings: stubSettings{newsEnabled: false}})
	if _, err := svc.Publish(context.Background(), testAnnID); !errors.Is(err, ErrNewsDisabled) {
		t.Fatalf("expected ErrNewsDisabled, got %v", err)
	}
}

func TestService_Publish_ExpiredDraft(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	a := draft()
	a.ExpiresAt = &past
	repo := &mockRepo{findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return a, nil }}
	svc := NewService(ServiceConfig{Repo: repo, Settings: stubSettings{newsEnabled: true}})
	if _, err := svc.Publish(context.Background(), testAnnID); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for expired draft, got %v", err)
	}
}

func TestService_Publish_NotFound(t *testing.T) {
	repo := &mockRepo{findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return nil, nil }}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.Publish(context.Background(), testAnnID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- Unpublish --------------------------------------------------------------

func TestService_Unpublish(t *testing.T) {
	var setNil, cancelled bool
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return published(), nil },
		setPublishedFn: func(_ context.Context, _ int64, publishedAt *time.Time) error {
			setNil = publishedAt == nil
			return nil
		},
		listTargetsFn: func(_ context.Context, _ int64) ([]*usersModels.ParentAnnouncementTarget, error) { return nil, nil },
	}
	outbox := &stubOutbox{cancelFn: func(_ context.Context, _ string, _ int64, _ string) (int64, error) { cancelled = true; return 1, nil }}
	svc := NewService(ServiceConfig{Repo: repo, Outbox: outbox})
	if _, err := svc.Unpublish(context.Background(), testAnnID); err != nil {
		t.Fatalf("Unpublish: %v", err)
	}
	if !setNil || !cancelled {
		t.Fatalf("expected published_at cleared and e-mails cancelled, got setNil=%v cancelled=%v", setNil, cancelled)
	}
}

func TestService_Unpublish_Draft_NoOp(t *testing.T) {
	repo := &mockRepo{
		findByIDFn:    func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil },
		listTargetsFn: func(_ context.Context, _ int64) ([]*usersModels.ParentAnnouncementTarget, error) { return nil, nil },
	}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.Unpublish(context.Background(), testAnnID); err != nil {
		t.Fatalf("Unpublish draft: %v", err)
	}
}

func TestService_Unpublish_NotFound(t *testing.T) {
	repo := &mockRepo{findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return nil, nil }}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.Unpublish(context.Background(), testAnnID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- Stats & Recipients -----------------------------------------------------

func TestService_Stats(t *testing.T) {
	want := &usersModels.AnnouncementStats{TargetCount: 10, ReadCount: 4, AcknowledgedCount: 1}
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil },
		statsFn:    func(_ context.Context, _, _ int64) (*usersModels.AnnouncementStats, error) { return want, nil },
	}
	svc := NewService(ServiceConfig{Repo: repo})
	got, err := svc.Stats(context.Background(), testAnnID)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected stats: %+v", got)
	}
}

func TestService_Stats_NotFound(t *testing.T) {
	repo := &mockRepo{findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return nil, nil }}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.Stats(context.Background(), testAnnID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestService_Recipients(t *testing.T) {
	want := []*usersModels.AnnouncementRecipientStatus{{AccountID: 1, FirstName: "Ada", LastName: "L"}}
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil },
		audienceFn: func(_ context.Context, _, _ int64) ([]*usersModels.AnnouncementRecipientStatus, error) {
			return want, nil
		},
	}
	svc := NewService(ServiceConfig{Repo: repo})
	got, err := svc.Recipients(context.Background(), testAnnID)
	if err != nil {
		t.Fatalf("Recipients: %v", err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("unexpected recipients: %+v", got)
	}
}

func TestService_Recipients_NotFound(t *testing.T) {
	repo := &mockRepo{findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return nil, nil }}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.Recipients(context.Background(), testAnnID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- e-mail edge cases ------------------------------------------------------

func TestService_Publish_EmailNoOutbox(t *testing.T) {
	// send_email is on but no outbox is wired: publish still succeeds (in-app
	// feed works), the missing binding is logged, not fatal.
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) {
			a := draft()
			a.SendEmail = true
			return a, nil
		},
		publishIfDraft: func(_ context.Context, _ int64, _ time.Time) (bool, error) { return true, nil },
		listTargetsFn:  func(_ context.Context, _ int64) ([]*usersModels.ParentAnnouncementTarget, error) { return nil, nil },
	}
	svc := NewService(ServiceConfig{Repo: repo, Settings: stubSettings{newsEnabled: true}})
	if _, err := svc.Publish(context.Background(), testAnnID); err != nil {
		t.Fatalf("Publish without outbox: %v", err)
	}
}

func TestService_Get_RepoError(t *testing.T) {
	repo := &mockRepo{findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) {
		return nil, errors.New("db")
	}}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.Get(context.Background(), testAnnID); err == nil {
		t.Fatal("expected error")
	}
}

func TestService_Delete_RepoError(t *testing.T) {
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil },
		deleteFn:   func(_ context.Context, _ int64) error { return errors.New("delete failed") },
	}
	// no outbox -> cancelPendingEmails is a no-op, so the delete error surfaces.
	svc := NewService(ServiceConfig{Repo: repo})
	if err := svc.Delete(context.Background(), testAnnID); err == nil {
		t.Fatal("expected delete error")
	}
}

func TestService_Stats_RepoError(t *testing.T) {
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil },
		statsFn: func(_ context.Context, _, _ int64) (*usersModels.AnnouncementStats, error) {
			return nil, errors.New("db")
		},
	}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.Stats(context.Background(), testAnnID); err == nil {
		t.Fatal("expected error")
	}
}

func TestService_Recipients_RepoError(t *testing.T) {
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) { return draft(), nil },
		audienceFn: func(_ context.Context, _, _ int64) ([]*usersModels.AnnouncementRecipientStatus, error) {
			return nil, errors.New("db")
		},
	}
	svc := NewService(ServiceConfig{Repo: repo})
	if _, err := svc.Recipients(context.Background(), testAnnID); err == nil {
		t.Fatal("expected error")
	}
}

func TestService_Create_NilSettings(t *testing.T) {
	// A nil settings service means the feature cannot be confirmed on, so
	// newsEnabled returns false and Create refuses.
	svc := NewService(ServiceConfig{Repo: &mockRepo{}})
	if _, err := svc.Create(context.Background(), 1, validInputM()); !errors.Is(err, ErrNewsDisabled) {
		t.Fatalf("expected ErrNewsDisabled with nil settings, got %v", err)
	}
}

func TestService_Publish_LogoLookupError_StillEnqueues(t *testing.T) {
	// A failed school-logo lookup is cosmetic: the publish e-mail still goes out,
	// just without the school logo.
	var enqueued int
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) {
			a := draft()
			a.SendEmail = true
			return a, nil
		},
		publishIfDraft: func(_ context.Context, _ int64, _ time.Time) (bool, error) { return true, nil },
		listTargetsFn:  func(_ context.Context, _ int64) ([]*usersModels.ParentAnnouncementTarget, error) { return nil, nil },
		resolveEmailsFn: func(_ context.Context, _, _ int64) ([]*usersModels.AnnouncementRecipient, error) {
			return []*usersModels.AnnouncementRecipient{{Email: "a@b.de"}}, nil
		},
		schoolNameFn: func(_ context.Context, _ int64) (string, error) { return "OGS", nil },
	}
	outbox := &stubOutbox{enqueueFn: func(_ context.Context, req platformService.EnqueueRequest) (*platformModels.EmailOutbox, error) {
		enqueued++
		if req.Payload[emailPayloadLogoURL] != "" {
			t.Fatalf("expected empty logo URL after lookup error, got %v", req.Payload[emailPayloadLogoURL])
		}
		return &platformModels.EmailOutbox{}, nil
	}}
	svc := NewService(ServiceConfig{
		Repo:     repo,
		Settings: stubSettings{newsEnabled: true, loginImageErr: errors.New("no logo")},
		Outbox:   outbox,
	})
	if _, err := svc.Publish(context.Background(), testAnnID); err != nil {
		t.Fatalf("Publish with logo error: %v", err)
	}
	if enqueued != 1 {
		t.Fatalf("expected 1 e-mail, got %d", enqueued)
	}
}

func TestService_Publish_EmptyAudience(t *testing.T) {
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) {
			a := draft()
			a.SendEmail = true
			return a, nil
		},
		publishIfDraft:  func(_ context.Context, _ int64, _ time.Time) (bool, error) { return true, nil },
		listTargetsFn:   func(_ context.Context, _ int64) ([]*usersModels.ParentAnnouncementTarget, error) { return nil, nil },
		resolveEmailsFn: func(_ context.Context, _, _ int64) ([]*usersModels.AnnouncementRecipient, error) { return nil, nil },
	}
	outbox := &stubOutbox{enqueueFn: func(_ context.Context, _ platformService.EnqueueRequest) (*platformModels.EmailOutbox, error) {
		t.Fatal("must not enqueue for empty audience")
		return nil, nil
	}}
	svc := NewService(ServiceConfig{Repo: repo, Settings: stubSettings{newsEnabled: true}, Outbox: outbox})
	if _, err := svc.Publish(context.Background(), testAnnID); err != nil {
		t.Fatalf("Publish empty audience: %v", err)
	}
}

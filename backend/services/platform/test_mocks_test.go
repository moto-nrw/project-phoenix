package platform_test

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/platform"
)

// Shared mock for operator repository
type mockOperatorRepo struct {
	findByIDFn        func(ctx context.Context, id int64) (*platform.Operator, error)
	findByEmailFn     func(ctx context.Context, email string) (*platform.Operator, error)
	updateFn          func(ctx context.Context, operator *platform.Operator) error
	updateLastLoginFn func(ctx context.Context, id int64) error
	listFn            func(ctx context.Context) ([]*platform.Operator, error)
}

func (m *mockOperatorRepo) Create(ctx context.Context, operator *platform.Operator) error {
	return nil
}

func (m *mockOperatorRepo) FindByID(ctx context.Context, id int64) (*platform.Operator, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockOperatorRepo) FindByEmail(ctx context.Context, email string) (*platform.Operator, error) {
	if m.findByEmailFn != nil {
		return m.findByEmailFn(ctx, email)
	}
	return nil, nil
}

func (m *mockOperatorRepo) Update(ctx context.Context, operator *platform.Operator) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, operator)
	}
	return nil
}

func (m *mockOperatorRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *mockOperatorRepo) List(ctx context.Context) ([]*platform.Operator, error) {
	if m.listFn != nil {
		return m.listFn(ctx)
	}
	return []*platform.Operator{}, nil
}

func (m *mockOperatorRepo) UpdateLastLogin(ctx context.Context, id int64) error {
	if m.updateLastLoginFn != nil {
		return m.updateLastLoginFn(ctx, id)
	}
	return nil
}

// Shared mock for audit log repository
type mockAuditLogRepoShared struct {
	createFn func(ctx context.Context, entry *platform.OperatorAuditLog) error
}

func (m *mockAuditLogRepoShared) Create(ctx context.Context, entry *platform.OperatorAuditLog) error {
	if m.createFn != nil {
		return m.createFn(ctx, entry)
	}
	return nil
}

func (m *mockAuditLogRepoShared) FindByOperatorID(ctx context.Context, operatorID int64, limit int) ([]*platform.OperatorAuditLog, error) {
	return nil, nil
}

func (m *mockAuditLogRepoShared) FindByResourceType(ctx context.Context, resourceType string, limit int) ([]*platform.OperatorAuditLog, error) {
	return nil, nil
}

func (m *mockAuditLogRepoShared) FindByDateRange(ctx context.Context, start, end time.Time, limit int) ([]*platform.OperatorAuditLog, error) {
	return nil, nil
}

// Shared mock for announcement repository
type mockAnnouncementRepoShared struct {
	createFn    func(ctx context.Context, announcement *platform.Announcement) error
	findByIDFn  func(ctx context.Context, id int64) (*platform.Announcement, error)
	updateFn    func(ctx context.Context, announcement *platform.Announcement) error
	deleteFn    func(ctx context.Context, id int64) error
	listFn      func(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error)
	publishFn   func(ctx context.Context, id int64) error
	unpublishFn func(ctx context.Context, id int64) error
}

func (m *mockAnnouncementRepoShared) Create(ctx context.Context, announcement *platform.Announcement) error {
	if m.createFn != nil {
		return m.createFn(ctx, announcement)
	}
	return nil
}

func (m *mockAnnouncementRepoShared) FindByID(ctx context.Context, id int64) (*platform.Announcement, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockAnnouncementRepoShared) Update(ctx context.Context, announcement *platform.Announcement) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, announcement)
	}
	return nil
}

func (m *mockAnnouncementRepoShared) Delete(ctx context.Context, id int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockAnnouncementRepoShared) List(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
	if m.listFn != nil {
		return m.listFn(ctx, includeInactive)
	}
	return []*platform.Announcement{}, nil
}

func (m *mockAnnouncementRepoShared) ListPublished(ctx context.Context) ([]*platform.Announcement, error) {
	return []*platform.Announcement{}, nil
}

func (m *mockAnnouncementRepoShared) Publish(ctx context.Context, id int64) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, id)
	}
	return nil
}

func (m *mockAnnouncementRepoShared) Unpublish(ctx context.Context, id int64) error {
	if m.unpublishFn != nil {
		return m.unpublishFn(ctx, id)
	}
	return nil
}

// Shared mock for announcement view repository
type mockAnnouncementViewRepoShared struct {
	getUnreadForUserFn func(ctx context.Context, userID int64, userRoles []string) ([]*platform.Announcement, error)
	countUnreadFn      func(ctx context.Context, userID int64, userRoles []string) (int, error)
	markSeenFn         func(ctx context.Context, userID, announcementID int64) error
	markDismissedFn    func(ctx context.Context, userID, announcementID int64) error
	getStatsFn         func(ctx context.Context, announcementID int64) (*platform.AnnouncementStats, error)
	getViewDetailsFn   func(ctx context.Context, announcementID int64) ([]*platform.AnnouncementViewDetail, error)
}

func (m *mockAnnouncementViewRepoShared) MarkSeen(ctx context.Context, userID, announcementID int64) error {
	if m.markSeenFn != nil {
		return m.markSeenFn(ctx, userID, announcementID)
	}
	return nil
}

func (m *mockAnnouncementViewRepoShared) MarkDismissed(ctx context.Context, userID, announcementID int64) error {
	if m.markDismissedFn != nil {
		return m.markDismissedFn(ctx, userID, announcementID)
	}
	return nil
}

func (m *mockAnnouncementViewRepoShared) GetUnreadForUser(ctx context.Context, userID int64, userRoles []string) ([]*platform.Announcement, error) {
	if m.getUnreadForUserFn != nil {
		return m.getUnreadForUserFn(ctx, userID, userRoles)
	}
	return []*platform.Announcement{}, nil
}

func (m *mockAnnouncementViewRepoShared) CountUnread(ctx context.Context, userID int64, userRoles []string) (int, error) {
	if m.countUnreadFn != nil {
		return m.countUnreadFn(ctx, userID, userRoles)
	}
	return 0, nil
}

func (m *mockAnnouncementViewRepoShared) HasSeen(ctx context.Context, userID, announcementID int64) (bool, error) {
	return false, nil
}

func (m *mockAnnouncementViewRepoShared) GetStats(ctx context.Context, announcementID int64) (*platform.AnnouncementStats, error) {
	if m.getStatsFn != nil {
		return m.getStatsFn(ctx, announcementID)
	}
	return &platform.AnnouncementStats{}, nil
}

func (m *mockAnnouncementViewRepoShared) GetViewDetails(ctx context.Context, announcementID int64) ([]*platform.AnnouncementViewDetail, error) {
	if m.getViewDetailsFn != nil {
		return m.getViewDetailsFn(ctx, announcementID)
	}
	return []*platform.AnnouncementViewDetail{}, nil
}

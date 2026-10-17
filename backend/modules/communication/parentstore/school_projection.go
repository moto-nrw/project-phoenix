package parentstore

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// SchoolDirectory names the schools of the inbox rows and the announcement
// feed. Organisation & Tenancy owns the schools; the composition root binds
// it.
type SchoolDirectory interface {
	// SchoolName is the name of one school.
	SchoolName(ctx context.Context, tenantID int64) (string, error)
	// SchoolNames names the schools with the given IDs; an unknown school is
	// absent.
	SchoolNames(ctx context.Context, ids []int64) (map[int64]string, error)
}

// WithInboxSchoolNames names the school of every inbox row through schools.
func WithInboxSchoolNames(reads ParentMessageReadRepository, schools SchoolDirectory) ParentMessageReadRepository {
	return schoolParentMessageReadRepository{ParentMessageReadRepository: reads, schools: schools}
}

// WithAnnouncementSchoolNames serves the announcement repository's school
// names through schools.
func WithAnnouncementSchoolNames(announcements ParentAnnouncementRepository, schools SchoolDirectory) ParentAnnouncementRepository {
	return schoolParentAnnouncementRepository{ParentAnnouncementRepository: announcements, schools: schools}
}

type schoolParentAnnouncementRepository struct {
	ParentAnnouncementRepository
	schools SchoolDirectory
}

func (r schoolParentAnnouncementRepository) AnnouncementSchoolName(ctx context.Context, tenantID int64) (string, error) {
	return r.SchoolName(ctx, tenantID)
}

func (r schoolParentAnnouncementRepository) SchoolName(ctx context.Context, tenantID int64) (string, error) {
	name, err := r.schools.SchoolName(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve school name: %w", err)
	}
	return name, nil
}

func (r schoolParentAnnouncementRepository) ListFeedForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) ([]*communication.PortalAnnouncementFeedItem, error) {
	items, err := r.ParentAnnouncementRepository.ListFeedForAccount(ctx, accountID, scope)
	if err != nil || len(items) == 0 {
		return items, err
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.TenantID)
	}
	names, err := r.schools.SchoolNames(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load schools for parent announcement feed: %w", err)
	}
	for _, item := range items {
		name, found := names[item.TenantID]
		if !found {
			return nil, fmt.Errorf("school %d missing for parent announcement feed", item.TenantID)
		}
		item.SchoolName = name
	}
	return items, nil
}

type schoolParentMessageReadRepository struct {
	ParentMessageReadRepository
	schools SchoolDirectory
}

func (r schoolParentMessageReadRepository) ListInboxForStaff(ctx context.Context, accountID int64, allStudents, onlyUnread bool) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListInboxForStaff(ctx, accountID, allStudents, onlyUnread)
	return enrichInboxSchools(ctx, r.schools, rows, err)
}

func (r schoolParentMessageReadRepository) ListThreadsForStudent(ctx context.Context, accountID, studentID int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListThreadsForStudent(ctx, accountID, studentID)
	return enrichInboxSchools(ctx, r.schools, rows, err)
}

func (r schoolParentMessageReadRepository) ListThreadsForGuardianStudent(ctx context.Context, accountID, studentID int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListThreadsForGuardianStudent(ctx, accountID, studentID)
	return enrichInboxSchools(ctx, r.schools, rows, err)
}

func (r schoolParentMessageReadRepository) ListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListThreadsForGuardianTenants(ctx, accountID, tenantIDs)
	return enrichInboxSchools(ctx, r.schools, rows, err)
}

func enrichInboxSchools(ctx context.Context, schools SchoolDirectory, rows []*communication.PortalInboxThread, queryErr error) ([]*communication.PortalInboxThread, error) {
	if queryErr != nil || len(rows) == 0 {
		return rows, queryErr
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.TenantID)
	}
	names, err := schools.SchoolNames(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load schools for parent message inbox: %w", err)
	}
	for _, row := range rows {
		name, found := names[row.TenantID]
		if !found {
			return nil, fmt.Errorf("school %d missing for parent message inbox", row.TenantID)
		}
		row.SchoolName = name
	}
	return rows, nil
}

// MessagingRepositories are the parent-messaging and announcement
// repositories a composition root hands to the retained services, next to
// the staff-messaging read cursors.
type MessagingRepositories struct {
	ParentAnnouncement  ParentAnnouncementRepository
	ParentMessageThread ParentMessageThreadRepository
	ParentMessageRead   ParentMessageReadRepository
	StaffMessageRead    communication.StaffReadRecords
}

// WithSchoolNames returns the repositories with the announcement and inbox
// school names served through schools.
func (r MessagingRepositories) WithSchoolNames(schools SchoolDirectory) MessagingRepositories {
	return MessagingRepositories{
		ParentAnnouncement:  WithAnnouncementSchoolNames(r.ParentAnnouncement, schools),
		ParentMessageThread: r.ParentMessageThread,
		ParentMessageRead:   WithInboxSchoolNames(r.ParentMessageRead, schools),
		StaffMessageRead:    r.StaffMessageRead,
	}
}

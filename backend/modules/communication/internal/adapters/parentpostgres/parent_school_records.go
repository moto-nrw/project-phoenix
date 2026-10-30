package parentpostgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"
)

// WithInboxSchoolNames names the school of every inbox row through schools.
func WithInboxSchoolNames(reads recordports.ParentMessageReadRepository, schools recordports.SchoolDirectory) recordports.ParentMessageReadRepository {
	return schoolParentMessageReadRepository{ParentMessageReadRepository: reads, schools: schools}
}

// WithAnnouncementSchoolNames serves the announcement repository's school
// names through schools.
func WithAnnouncementSchoolNames(announcements recordports.ParentAnnouncementRepository, schools recordports.SchoolDirectory) recordports.ParentAnnouncementRepository {
	return schoolParentAnnouncementRepository{ParentAnnouncementRepository: announcements, schools: schools}
}

type schoolParentAnnouncementRepository struct {
	recordports.ParentAnnouncementRepository
	schools recordports.SchoolDirectory
}

// SurveyRecords keeps the survey store of the wrapped announcements.
func (r schoolParentAnnouncementRepository) SurveyRecords() communication.SurveyRecords {
	if source, ok := r.ParentAnnouncementRepository.(interface {
		SurveyRecords() communication.SurveyRecords
	}); ok {
		return source.SurveyRecords()
	}
	return nil
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
	recordports.ParentMessageReadRepository
	schools recordports.SchoolDirectory
}

func (r schoolParentMessageReadRepository) ListInboxForStaff(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach, onlyUnread bool) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListInboxForStaff(ctx, accountID, reach, onlyUnread)
	return enrichInboxSchools(ctx, r.schools, rows, err)
}
func (r schoolParentMessageReadRepository) ListThreadsForStudent(ctx context.Context, accountID, studentID int64, reach communication.ParentMessageStaffReach) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListThreadsForStudent(ctx, accountID, studentID, reach)
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
func enrichInboxSchools(ctx context.Context, schools recordports.SchoolDirectory, rows []*communication.PortalInboxThread, queryErr error) ([]*communication.PortalInboxThread, error) {
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

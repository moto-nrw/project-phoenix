package repositories

import (
	"context"
	"fmt"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// personAnnouncementViewRepository shows the viewer's person name and falls
// back to the account e-mail, as the previous join did.
type personAnnouncementViewRepository struct {
	platformModels.AnnouncementViewRepository
	persons peopledirectory.Query
}

func (r personAnnouncementViewRepository) GetViewDetails(ctx context.Context, announcementID int64) ([]*platformModels.AnnouncementViewDetail, error) {
	details, err := r.AnnouncementViewRepository.GetViewDetails(ctx, announcementID)
	if err != nil || len(details) == 0 {
		return details, err
	}
	ids := make([]int64, 0, len(details))
	for _, detail := range details {
		ids = append(ids, detail.UserID)
	}
	persons, err := personsByAccount(ctx, r.persons, ids)
	if err != nil {
		return nil, err
	}
	for _, detail := range details {
		detail.UserName = detail.AccountEmail
		if person, found := persons[detail.UserID]; found {
			if name := person.FullName(); name != " " {
				detail.UserName = name
			}
		}
	}
	return details, nil
}

// personOperatorSummariesRepository fills the person counts of the operator
// overviews from the platform-wide count the People Directory keeps.
type personOperatorSummariesRepository struct {
	platformModels.OperatorSummariesRepository
	persons peopledirectory.Query
	schools func() platformModels.SchoolRepository
}

func (r personOperatorSummariesRepository) OrganizationSummaries(ctx context.Context) ([]*platformModels.OrganizationSummary, error) {
	rows, err := r.OperatorSummariesRepository.OrganizationSummaries(ctx)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	counts, err := r.persons.CountPersonsByTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("count persons for organization summaries: %w", err)
	}
	schools, err := r.schools().ListNonDeleted(ctx)
	if err != nil {
		return nil, fmt.Errorf("load schools for organization summaries: %w", err)
	}
	byOrganization := make(map[int64]int, len(rows))
	for _, school := range schools {
		byOrganization[school.OrganizationID] += counts[school.ID]
	}
	for _, row := range rows {
		row.PersonenCount = byOrganization[row.ID]
	}
	return rows, nil
}

func (r personOperatorSummariesRepository) SchoolSummaries(ctx context.Context) ([]*platformModels.SchoolSummary, error) {
	rows, err := r.OperatorSummariesRepository.SchoolSummaries(ctx)
	if err != nil {
		return nil, err
	}
	return rows, r.attachSchoolPersonCounts(ctx, rows)
}

func (r personOperatorSummariesRepository) SchoolSummariesByOrganization(ctx context.Context, organizationID int64) ([]*platformModels.SchoolSummary, error) {
	rows, err := r.OperatorSummariesRepository.SchoolSummariesByOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	return rows, r.attachSchoolPersonCounts(ctx, rows)
}

func (r personOperatorSummariesRepository) attachSchoolPersonCounts(ctx context.Context, rows []*platformModels.SchoolSummary) error {
	if len(rows) == 0 {
		return nil
	}
	counts, err := r.persons.CountPersonsByTenant(ctx)
	if err != nil {
		return fmt.Errorf("count persons for school summaries: %w", err)
	}
	for _, row := range rows {
		row.PersonenCount = counts[row.ID]
	}
	return nil
}

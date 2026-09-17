package application

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// ListSchoolPersons lists the persons of a school. A soft-deleted school
// lists nobody.
func (p *Provisioning) ListSchoolPersons(ctx context.Context, schoolID int64) ([]organizationtenancy.OperatorPerson, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]organizationtenancy.OperatorPerson, error) {
		if _, err := p.findSchool(adminCtx, schoolID); err != nil {
			return nil, err
		}
		schools, err := p.dashboard.SchoolSummaries(adminCtx, nil)
		if err != nil {
			return nil, err
		}
		for _, school := range schools {
			if school.ID == schoolID && school.DeletedAt == nil {
				return p.listPersons(adminCtx, []domain.SchoolSummary{school})
			}
		}
		return []organizationtenancy.OperatorPerson{}, nil
	})
}

// ListOrganizationPersons lists the persons of every non-deleted school of an
// organisation.
func (p *Provisioning) ListOrganizationPersons(ctx context.Context, organizationID int64) ([]organizationtenancy.OperatorPerson, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]organizationtenancy.OperatorPerson, error) {
		if _, err := p.organizations.FindOrganization(adminCtx, organizationID); err != nil {
			return nil, mapOrganizationError(err, organizationID)
		}
		schools, err := p.dashboard.SchoolSummaries(adminCtx, &organizationID)
		if err != nil {
			return nil, err
		}
		live := make([]domain.SchoolSummary, 0, len(schools))
		for _, school := range schools {
			if school.DeletedAt == nil {
				live = append(live, school)
			}
		}
		return p.listPersons(adminCtx, live)
	})
}

func (p *Provisioning) listPersons(ctx context.Context, schools []domain.SchoolSummary) ([]organizationtenancy.OperatorPerson, error) {
	byID := make(map[int64]domain.SchoolSummary, len(schools))
	tenantIDs := make([]int64, 0, len(schools))
	for _, school := range schools {
		byID[school.ID] = school
		tenantIDs = append(tenantIDs, school.ID)
	}
	persons, err := p.people.ListPersons(ctx, tenantIDs)
	result := make([]organizationtenancy.OperatorPerson, 0, len(persons))
	for _, person := range persons {
		school, found := byID[person.TenantID]
		if !found {
			continue
		}
		result = append(result, organizationtenancy.OperatorPerson{
			ID: person.ID, FirstName: person.FirstName, LastName: person.LastName,
			HasAccount: person.HasAccount, AccountEmail: person.AccountEmail,
			HasRFIDCard: person.HasRFIDCard, IsStaff: person.IsStaff, IsStudent: person.IsStudent,
			SchoolID: school.ID, SchoolName: school.Name,
			OrganizationID: school.OrganizationID, OrganizationName: school.OrganizationName,
			CreatedAt: person.CreatedAt,
		})
	}
	return result, err
}

// SoftDeletePerson anonymises and soft-deletes a person of any school: the
// RFID card and the account are unlinked, the account is deactivated and its
// e-mail anonymised. A staff member who still supervises a group cannot be
// deleted.
func (p *Provisioning) SoftDeletePerson(ctx context.Context, personID int64, operatorID int64, clientIP net.IP) error {
	if personID <= 0 {
		return invalidData("person id is required")
	}
	return p.inAdmin(ctx, func(adminCtx context.Context) error {
		person, found, err := p.people.FindPerson(adminCtx, personID)
		if err != nil {
			return fmt.Errorf("SoftDeletePerson: find person: %w", err)
		}
		if !found {
			return &organizationtenancy.PersonNotFoundError{PersonID: personID}
		}
		if err := p.ensureNoActiveSupervision(adminCtx, personID); err != nil {
			return err
		}
		if person.HasRFIDCard {
			if err := p.people.UnlinkRFIDCard(adminCtx, personID); err != nil {
				return fmt.Errorf("SoftDeletePerson: unlink rfid: %w", err)
			}
		}
		if person.AccountID != nil {
			if err := p.retireAccount(adminCtx, personID, *person.AccountID); err != nil {
				return err
			}
		}
		if err := p.people.AnonymizeAndSoftDelete(adminCtx, personID); err != nil {
			return fmt.Errorf("SoftDeletePerson: anonymize and soft delete: %w", err)
		}
		p.logAction(adminCtx, operatorID, domain.AuditActionSoftDelete, domain.AuditResourcePerson, &personID, clientIP, map[string]any{
			"person_id": personID,
			"school_id": person.TenantID,
		})
		p.logger.Info("person_soft_deleted",
			slog.Int64("person_id", personID),
			slog.Int64("school_id", person.TenantID),
			slog.Int64("operator_id", operatorID),
		)
		return nil
	})
}

// ensureNoActiveSupervision refuses the deletion of a staff member with an
// active supervision. A failing staff or supervision lookup does not block
// the deletion.
func (p *Provisioning) ensureNoActiveSupervision(ctx context.Context, personID int64) error {
	staff, found, err := p.people.FindStaff(ctx, personID)
	if err != nil || !found {
		return nil
	}
	count, err := p.presence.CountActiveSupervisions(ctx, staff.TenantID, staff.ID)
	if err != nil {
		p.logger.Warn("soft_delete_supervision_check_failed",
			slog.Int64("person_id", personID),
			slog.Int64("staff_id", staff.ID),
			slog.Any("error", err),
		)
		return nil
	}
	if count > 0 {
		return &organizationtenancy.PersonHasActiveSupervisionsError{PersonID: personID, Count: count}
	}
	return nil
}

// retireAccount deactivates the person's account, anonymises its e-mail and
// unlinks it from the person. A failed deactivation is logged and does not
// stop the deletion.
func (p *Provisioning) retireAccount(ctx context.Context, personID, accountID int64) error {
	if err := p.identity.DeactivateAccount(ctx, accountID); err != nil {
		p.logger.Warn("soft_delete_account_deactivation_failed",
			slog.Int64("person_id", personID),
			slog.Int64("account_id", accountID),
			slog.Any("error", err),
		)
	}
	if err := p.identity.AnonymizeAccount(ctx, accountID, fmt.Sprintf("deleted-%d@anonymized.local", personID)); err != nil {
		return fmt.Errorf("SoftDeletePerson: anonymize account: %w", err)
	}
	if err := p.people.UnlinkAccount(ctx, personID); err != nil {
		return fmt.Errorf("SoftDeletePerson: unlink account: %w", err)
	}
	return nil
}

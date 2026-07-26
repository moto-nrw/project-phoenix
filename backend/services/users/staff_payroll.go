package users

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Personnel number update (#1417 Tranche 2b): the payroll system's
// Personalnummer is what DATEV bills against, so the write path has two hard
// rules: uniqueness per tenant (mapped to a stable conflict error) and no
// change without an audit row — the audit insert runs in the same tenant
// transaction, before the update, mirroring the tombstone contract of #2001.

// ErrPersonnelNumberTaken marks a Personalnummer already assigned to another
// staff member of the same tenant — HTTP 409.
var ErrPersonnelNumberTaken = errors.New("personnel number already assigned to another staff member")

// ErrPersonnelNumberInvalid marks a syntactically invalid Personalnummer — HTTP 400.
var ErrPersonnelNumberInvalid = errors.New("invalid personnel number")

// personnelNumberPattern accepts what both DATEV systems can match on:
// digits only. The length cap is deliberately looser than any single DATEV
// format — the writers (#1417 follow-up) enforce their exact per-format
// bounds; this layer only blocks obvious junk.
var personnelNumberPattern = regexp.MustCompile(`^\d{1,9}$`)

// UpdatePersonnelNumber sets or clears (value nil / empty) the staff
// member's Personalnummer. A no-op change writes no audit row.
func (s *personService) UpdatePersonnelNumber(ctx context.Context, staffID int64, value *string, changedByStaffID int64, note string) (*userModels.Staff, error) {
	if s.PersonnelNumberAudit == nil {
		return nil, fmt.Errorf("personnel number audit repository is not wired; refusing unaudited change")
	}
	if changedByStaffID <= 0 {
		return nil, fmt.Errorf("changed-by staff id is required")
	}

	normalized := normalizePersonnelNumber(value)
	if normalized != nil && !personnelNumberPattern.MatchString(*normalized) {
		return nil, ErrPersonnelNumberInvalid
	}

	var updated *userModels.Staff
	tenantID := tenant.FromContext(ctx)
	if err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var err error
		updated, err = s.updatePersonnelNumberInTx(ctx, staffID, normalized, changedByStaffID, note)
		return err
	}); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *personService) updatePersonnelNumberInTx(
	ctx context.Context,
	staffID int64,
	normalized *string,
	changedByStaffID int64,
	note string,
) (*userModels.Staff, error) {
	staff, err := s.StaffRepo.FindByIDForUpdate(ctx, staffID)
	if err != nil {
		return nil, err
	}

	oldValue := derefString(staff.PersonnelNumber)
	newValue := derefString(normalized)
	if oldValue == newValue {
		return staff, nil
	}

	if err := s.PersonnelNumberAudit.Create(ctx, &auditModels.PersonnelNumberChange{
		StaffID:   staff.ID,
		ChangedBy: changedByStaffID,
		OldValue:  oldValue,
		NewValue:  newValue,
		Note:      strings.TrimSpace(note),
	}); err != nil {
		return nil, fmt.Errorf("write personnel number audit: %w", err)
	}

	staff.PersonnelNumber = normalized
	if err := s.StaffRepo.Update(ctx, staff); err != nil {
		if base.IsUniqueViolationOn(err, userModels.PersonnelNumberUniqueConstraintName) {
			return nil, ErrPersonnelNumberTaken
		}
		return nil, err
	}
	return staff, nil
}

func normalizePersonnelNumber(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

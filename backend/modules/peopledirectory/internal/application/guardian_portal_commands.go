package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// The parents-portal row writes. Each normalizes its row with the rules every
// guardian write applies, refuses outside a tenant transaction, and writes in
// the caller's transaction (or one it opens for the tenant in context).
// Deciding whether the caller may write stays with the workflow.

func (s *GuardianService) CreateContact(ctx context.Context, contact domain.GuardianContact) (id int64, err error) {
	contact, err = domain.NormalizeGuardianContact(contact)
	if err != nil {
		return 0, err
	}
	if contact.PreferredContactMethod == "" {
		contact.PreferredContactMethod = domain.DefaultGuardianContactMethod
	}
	if contact.LanguagePreference == "" {
		contact.LanguagePreference = domain.DefaultGuardianLanguage
	}
	err = observeRun(ctx, s.observe, "create_guardian_contact", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		if s.tx.TenantID(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		var writeStats domain.OperationStats
		id, writeStats, err = s.store.InsertContact(txCtx, contact)
		stats.Add(writeStats)
		return err
	})
	return id, err
}

func (s *GuardianService) UpdateContact(ctx context.Context, guardianID int64, contact domain.GuardianContact) error {
	contact, err := domain.NormalizeGuardianContact(contact)
	if err != nil {
		return err
	}
	return observeRun(ctx, s.observe, "update_guardian_contact", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		if s.tx.TenantID(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		updated, writeStats, err := s.store.UpdateContact(txCtx, guardianID, contact)
		stats.Add(writeStats)
		if err == nil && !updated {
			return domain.ErrGuardianNotFound
		}
		return err
	})
}

// ReplacePhones validates every row before it deletes anything, so a refused
// row never leaves the guardian without numbers.
func (s *GuardianService) ReplacePhones(ctx context.Context, guardianID int64, phones []domain.GuardianPhoneRecord) error {
	normalized := make([]domain.GuardianPhoneRecord, 0, len(phones))
	for _, phone := range phones {
		value, err := domain.NormalizeGuardianPhone(phone)
		if err != nil {
			return err
		}
		normalized = append(normalized, value)
	}
	return observeRun(ctx, s.observe, "replace_guardian_phones", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		if s.tx.TenantID(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		deleteStats, err := s.store.DeletePhones(txCtx, guardianID)
		stats.Add(deleteStats)
		if err != nil {
			return err
		}
		for _, phone := range normalized {
			_, insertStats, err := s.store.InsertPhone(txCtx, guardianID, phone)
			stats.Add(insertStats)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *GuardianService) AddPhone(ctx context.Context, guardianID int64, phone domain.GuardianPhoneRecord) (id int64, err error) {
	phone, err = domain.NormalizeGuardianPhone(phone)
	if err != nil {
		return 0, err
	}
	err = observeRun(ctx, s.observe, "add_guardian_phone_record", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		if s.tx.TenantID(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		var writeStats domain.OperationStats
		id, writeStats, err = s.store.InsertPhone(txCtx, guardianID, phone)
		stats.Add(writeStats)
		return err
	})
	return id, err
}

func (s *GuardianService) SetPhoneNumber(ctx context.Context, phoneID int64, number string) error {
	number, err := domain.NormalizeGuardianPhoneNumber(number)
	if err != nil {
		return err
	}
	return observeRun(ctx, s.observe, "set_guardian_phone_number", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		if s.tx.TenantID(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		found, writeStats, err := s.store.UpdatePhoneNumber(txCtx, phoneID, number)
		stats.Add(writeStats)
		if err == nil && !found {
			return domain.ErrGuardianPhoneNotFound
		}
		return err
	})
}

func (s *GuardianService) DeletePhone(ctx context.Context, phoneID int64) error {
	return observeRun(ctx, s.observe, "delete_guardian_phone_record", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		if s.tx.TenantID(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		found, writeStats, err := s.store.DeletePhone(txCtx, phoneID)
		stats.Add(writeStats)
		if err == nil && !found {
			return domain.ErrGuardianPhoneNotFound
		}
		return err
	})
}

func (s *GuardianService) LinkContact(ctx context.Context, link domain.GuardianLinkRecord) (linkID int64, err error) {
	link, err = domain.NormalizeGuardianLink(link)
	if err != nil {
		return 0, err
	}
	err = observeRun(ctx, s.observe, "link_guardian_contact", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		if s.tx.TenantID(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		if s.owners == nil {
			return domain.ErrGuardianLinkOwnersUnbound
		}
		// The savepoint makes the three halves one write even when the caller
		// handles a failure and commits its transaction.
		return s.tx.RunSavepoint(txCtx, func(ctx context.Context) error {
			linkID, err = s.writeLink(ctx, link, stats)
			return err
		})
	})
	if err != nil {
		linkID = 0
	}
	return linkID, err
}

// writeLink writes the relationship and, when it is new, its pickup
// permission and portal access.
func (s *GuardianService) writeLink(ctx context.Context, link domain.GuardianLinkRecord, stats *domain.OperationStats) (int64, error) {
	tenantID := s.tx.TenantID(ctx)
	linkID, writeStats, err := s.store.InsertLinkIfAbsent(ctx, link)
	stats.Add(writeStats)
	if err != nil || linkID == 0 {
		return linkID, err
	}
	if err := s.owners.CreateGuardianPickupPermission(ctx, tenantID, linkID, link.CanPickup, link.PickupNotes); err != nil {
		return linkID, err
	}
	accountID, readStats, err := s.store.GuardianAccount(ctx, link.GuardianProfileID)
	stats.Add(readStats)
	if err != nil {
		return linkID, err
	}
	return linkID, s.owners.GrantGuardianStudentAccess(ctx, tenantID, linkID, accountID, link.Permissions)
}

// PatchLinkPickup writes exactly the supplied columns: the emergency contact
// flag on the relationship, the pickup flag and note on Care Plan's pickup
// permission, under the relationship row lock.
func (s *GuardianService) PatchLinkPickup(ctx context.Context, patch domain.GuardianLinkPickupPatch) (affected int64, err error) {
	if patch.LinkID <= 0 || (patch.CanPickup == nil && patch.IsEmergencyContact == nil && !patch.SetPickupNotes) {
		return 0, &domain.GuardianInvalidError{Reason: "relationship ID and at least one pickup column are required"}
	}
	err = observeRun(ctx, s.observe, "patch_guardian_link_pickup", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		if s.tx.TenantID(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		if s.owners == nil {
			return domain.ErrGuardianLinkOwnersUnbound
		}
		return s.tx.RunSavepoint(txCtx, func(ctx context.Context) error {
			affected, err = s.writeLinkPickup(ctx, patch, stats)
			return err
		})
	})
	if err != nil {
		affected = 0
	}
	return affected, err
}

// writeLinkPickup locks the relationship, sets its emergency flag when one is
// supplied and hands the pickup columns to Care Plan.
func (s *GuardianService) writeLinkPickup(ctx context.Context, patch domain.GuardianLinkPickupPatch, stats *domain.OperationStats) (int64, error) {
	found, writeStats, err := s.store.LockLinkForPickup(ctx, patch.LinkID, patch.IsEmergencyContact)
	stats.Add(writeStats)
	if err != nil || !found {
		return 0, err
	}
	if patch.CanPickup == nil && !patch.SetPickupNotes {
		return 1, nil
	}
	_, err = s.owners.ChangeGuardianPickupPermission(ctx, s.tx.TenantID(ctx), patch.LinkID, patch.CanPickup, patch.SetPickupNotes, patch.PickupNotes)
	return 1, err
}

// SetPortalLocale only ever touches the tenant in context: parent accounts
// span schools, and the caller decides per school. Which locales are
// supported is the caller's rule, so the value is written as given.
func (s *GuardianService) SetPortalLocale(ctx context.Context, accountID int64, locale string) (updated int64, err error) {
	if accountID <= 0 {
		return 0, &domain.GuardianInvalidError{Reason: "account ID is required"}
	}
	err = observeRun(ctx, s.observe, "set_guardian_portal_locale", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		if s.tx.TenantID(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		var writeStats domain.OperationStats
		updated, writeStats, err = s.store.SetPortalLocale(txCtx, accountID, locale)
		stats.Add(writeStats)
		return err
	})
	return updated, err
}

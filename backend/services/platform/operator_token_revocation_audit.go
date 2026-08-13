package platform

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/auth/rotation"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

func (s *operatorAuthService) auditOperatorRevocation(ctx context.Context, operatorID int64, familyID, reason string, count int) error {
	entry := &platformModels.OperatorAuditLog{
		OperatorID:   operatorID,
		Action:       platformModels.ActionTokenRevoked,
		ResourceType: platformModels.ResourceOperator,
		ResourceID:   &operatorID,
	}
	if err := entry.SetChanges(map[string]any{
		"portal_scope":        "operator",
		"family_fingerprint":  rotation.FamilyFingerprint(familyID),
		"reason":              reason,
		"revoked_token_count": count,
	}); err != nil {
		return fmt.Errorf("encode operator token revocation audit: %w", err)
	}
	if err := s.AuditLogRepo.Create(ctx, entry); err != nil {
		return fmt.Errorf("audit operator token revocation: %w", err)
	}
	return nil
}

func (s *operatorAuthService) deleteOperatorFamilyWithAudit(ctx context.Context, token *platformModels.OperatorRefreshToken, reason string) error {
	deleted, err := s.RefreshTokenRepo.DeleteByFamilyIDReturning(ctx, token.FamilyID)
	if err != nil {
		return err
	}
	return s.auditOperatorRevocation(ctx, token.OperatorID, token.FamilyID, reason, len(deleted))
}

func (s *operatorAuthService) deleteAllOperatorTokensWithAudit(ctx context.Context, operatorID int64, reason string) error {
	deleted, err := s.RefreshTokenRepo.DeleteByOperatorIDReturning(ctx, operatorID)
	if err != nil {
		return err
	}
	counts := make(map[string]int)
	for _, token := range deleted {
		counts[token.FamilyID]++
	}
	for familyID, count := range counts {
		if err := s.auditOperatorRevocation(ctx, operatorID, familyID, reason, count); err != nil {
			return err
		}
	}
	return nil
}

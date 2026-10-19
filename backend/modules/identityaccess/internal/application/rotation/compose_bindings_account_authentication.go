package rotation

import (
	"context"
	"time"
)

type RotationPolicy struct{}

func (RotationPolicy) RecoveryGrace() time.Duration { return RecoveryGrace }
func (RotationPolicy) MaxRecoveryHops() int         { return MaxRecoveryHops }
func (RotationPolicy) RecoveryProofHash(ctx context.Context) []byte {
	return RecoveryProofHash(ctx)
}
func (RotationPolicy) MatchesRecoveryProof(ctx context.Context, expected []byte) bool {
	return MatchesRecoveryProof(ctx, expected)
}
func (RotationPolicy) FamilyFingerprint(familyID string) string {
	return FamilyFingerprint(familyID)
}

package rotation

import "time"

// RotationPolicy binds the rotation bounds to the owner's port.
type RotationPolicy struct{}

func (RotationPolicy) RecoveryGrace() time.Duration { return RecoveryGrace }
func (RotationPolicy) MaxRecoveryHops() int         { return MaxRecoveryHops }
func (RotationPolicy) FamilyFingerprint(familyID string) string {
	return FamilyFingerprint(familyID)
}

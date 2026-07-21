package platform

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// OperatorRefreshToken is the server-side session handle behind an operator
// refresh JWT. The JWT carries Token as an opaque claim; refresh succeeds only
// while the matching row exists and is current.
type OperatorRefreshToken struct {
	base.Model `bun:"schema:platform,table:operator_refresh_tokens"`

	OperatorID        int64      `bun:"operator_id,notnull" json:"operator_id"`
	Token             string     `bun:"token,notnull,unique" json:"-"`
	Expiry            time.Time  `bun:"expiry,notnull" json:"expiry"`
	FamilyID          string     `bun:"family_id,notnull" json:"family_id"`
	Generation        int        `bun:"generation,notnull,default:0" json:"generation"`
	RotatedAt         *time.Time `bun:"rotated_at" json:"-"`
	ReplacementToken  *string    `bun:"replacement_token" json:"-"`
	RecoveryProofHash []byte     `bun:"recovery_proof_hash" json:"-"`
}

func (t *OperatorRefreshToken) Validate() error {
	if t.OperatorID <= 0 {
		return errors.New("operator ID is required")
	}
	if t.Token == "" {
		return errors.New("token value is required")
	}
	if t.Expiry.IsZero() {
		return errors.New("expiry is required")
	}
	if t.FamilyID == "" {
		return errors.New("family ID is required")
	}
	if (t.RotatedAt == nil) != (t.ReplacementToken == nil) {
		return errors.New("rotation handoff must include both timestamp and replacement token")
	}
	if t.RotatedAt == nil && len(t.RecoveryProofHash) != 0 {
		return errors.New("recovery proof hash requires a rotation handoff")
	}
	return nil
}

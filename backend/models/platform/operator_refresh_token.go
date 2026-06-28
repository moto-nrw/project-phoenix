package platform

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const tablePlatformOperatorRefreshTokens = "platform.operator_refresh_tokens"

// OperatorRefreshToken is the server-side session handle behind an operator
// refresh JWT. The JWT carries Token as an opaque claim; refresh succeeds only
// while the matching row exists and is current.
type OperatorRefreshToken struct {
	base.Model `bun:"schema:platform,table:operator_refresh_tokens"`

	OperatorID int64     `bun:"operator_id,notnull" json:"operator_id"`
	Token      string    `bun:"token,notnull,unique" json:"-"`
	Expiry     time.Time `bun:"expiry,notnull" json:"expiry"`
	FamilyID   string    `bun:"family_id,notnull" json:"family_id"`
	Generation int       `bun:"generation,notnull,default:0" json:"generation"`

	Operator *Operator `bun:"rel:belongs-to,join:operator_id=id" json:"operator,omitempty"`
}

func (t *OperatorRefreshToken) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`platform.operator_refresh_tokens AS "operator_refresh_token"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`platform.operator_refresh_tokens AS "operator_refresh_token"`)
	}
	return nil
}

func (t *OperatorRefreshToken) TableName() string {
	return tablePlatformOperatorRefreshTokens
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
	return nil
}

func (t *OperatorRefreshToken) GetID() any {
	return t.ID
}

func (t *OperatorRefreshToken) GetCreatedAt() time.Time {
	return t.CreatedAt
}

func (t *OperatorRefreshToken) GetUpdatedAt() time.Time {
	return t.UpdatedAt
}

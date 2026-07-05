package platform

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tablePlatformOperatorEmailChangeTokens is the schema-qualified table name
const tablePlatformOperatorEmailChangeTokens = "platform.operator_email_change_tokens"

// OperatorEmailChangeToken represents a pending email change verification token
type OperatorEmailChangeToken struct {
	base.Model      `bun:"schema:platform,table:operator_email_change_tokens"`
	OperatorID      int64      `bun:"operator_id,notnull" json:"operator_id"`
	NewEmail        string     `bun:"new_email,notnull" json:"new_email"`
	Token           string     `bun:"token,notnull" json:"token"`
	Expiry          time.Time  `bun:"expiry,notnull" json:"expiry"`
	Used            bool       `bun:"used,notnull,default:false" json:"used"`
	EmailSentAt     *time.Time `bun:"email_sent_at,nullzero" json:"email_sent_at,omitempty"`
	EmailError      *string    `bun:"email_error,nullzero" json:"email_error,omitempty"`
	EmailRetryCount int        `bun:"email_retry_count,notnull,default:0" json:"email_retry_count"`

	// Relations
	Operator *Operator `bun:"rel:belongs-to,join:operator_id=id" json:"operator,omitempty"`
}

// BeforeAppendModel sets the schema-qualified table expression for UPDATE and DELETE queries.
// This is NOT inherited from base.Model and must be explicitly implemented.
func (t *OperatorEmailChangeToken) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tablePlatformOperatorEmailChangeTokens)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tablePlatformOperatorEmailChangeTokens)
	}
	return nil
}

// Validate ensures the token data is valid
func (t *OperatorEmailChangeToken) Validate() error {
	if t.OperatorID <= 0 {
		return errors.New("operator ID is required")
	}
	if t.Token == "" {
		return errors.New("token value is required")
	}
	if t.NewEmail == "" {
		return errors.New("new email is required")
	}
	// Creation-time data-integrity guard: never persist an already-expired
	// token. This is distinct from the usage-time IsExpired decision (which
	// lives in the service); the repository Create path relies on it.
	if t.Expiry.Before(time.Now()) {
		return errors.New("token has already expired")
	}
	if t.Used {
		return errors.New("token has already been used")
	}
	return nil
}

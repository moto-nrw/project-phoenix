package audit

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
)

const (
	StudentConsentAGB            = "agb"
	StudentConsentDataProcessing = "data_processing"
	StudentConsentEmailContact   = "email_contact"
	StudentConsentPhoto          = "photo"

	StudentConsentGranted   = "granted"
	StudentConsentWithdrawn = "withdrawn"

	StudentConsentSourceEnrollment   = "enrollment"
	StudentConsentSourceTenantPortal = "tenant_portal"
	StudentConsentSourceParentPortal = "parent_portal"
	StudentConsentSourceImport       = "import"
	StudentConsentSourceMigration    = "migration_snapshot"
)

// StudentConsentChange is the append-only history of a student's consent
// state. The live timestamps on users.students remain the only current-state
// model. users.privacy_consents is intentionally not reused: it controls the
// retention window for visit data, not the enrollment acknowledgements and
// voluntary photo consent recorded here.
type StudentConsentChange struct {
	base.Model `bun:"schema:audit,table:student_consent_changes"`
	base.TenantModel
	StudentID      int64  `bun:"student_id,notnull" json:"student_id"`
	ConsentKey     string `bun:"consent_key,notnull" json:"consent_key"`
	Action         string `bun:"action,notnull" json:"action"`
	Source         string `bun:"source,notnull" json:"source"`
	ActorAccountID *int64 `bun:"actor_account_id" json:"actor_account_id,omitempty"`
}

type StudentConsentChangeRepository interface {
	Create(ctx context.Context, entry *StudentConsentChange) error
	ListByStudentID(ctx context.Context, studentID int64) ([]*StudentConsentChange, error)
}

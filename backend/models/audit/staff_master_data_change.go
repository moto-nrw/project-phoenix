package audit

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Stammdaten sections (#1423). Section names are part of the audit wire
// format and the API paths — keep them in sync with api/staff routes.
const (
	StammdatenSectionPerson         = "person"
	StammdatenSectionKontakt        = "kontakt"
	StammdatenSectionArbeitsvertrag = "arbeitsvertrag"
	StammdatenSectionQualifikation  = "qualifikationen"
	StammdatenSectionBankSteuer     = "bank-steuer"
)

// StaffMasterDataChange is an append-only audit row for one changed
// Stammdaten field of a staff member (#1423). Written in the same tenant
// transaction as the update — no change without a trace, same contract as
// audit.personnel_number_changes. For financial fields old/new carry the
// MASKED values only: the audit trail must not become a second store of
// IBAN / Steuer-ID / SV-Nummer outside the staff:financial gate. ChangedBy is
// normally a staff ID; bank-and-tax updates record the authenticated payroll
// account ID because that actor need not have a staff mapping.
type StaffMasterDataChange struct {
	bun.BaseModel `bun:"schema:audit,table:staff_master_data_changes"`
	ID            int64 `bun:"id,pk,autoincrement" json:"id"`
	base.TenantModel
	StaffID    int64     `bun:"staff_id,notnull" json:"staff_id"`
	ChangedBy  int64     `bun:"changed_by,notnull" json:"changed_by"`
	Section    string    `bun:"section,notnull" json:"section"`
	FieldName  string    `bun:"field_name,notnull" json:"field_name"`
	OldValue   string    `bun:"old_value,notnull" json:"old_value"`
	NewValue   string    `bun:"new_value,notnull" json:"new_value"`
	Note       string    `bun:"note,notnull" json:"note"`
	OccurredAt time.Time `bun:"occurred_at,nullzero,notnull,default:now()" json:"occurred_at"`
}

func (c *StaffMasterDataChange) Validate() error {
	if c.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if c.ChangedBy <= 0 {
		return errors.New("changed_by is required")
	}
	switch c.Section {
	case StammdatenSectionPerson, StammdatenSectionKontakt, StammdatenSectionArbeitsvertrag,
		StammdatenSectionQualifikation, StammdatenSectionBankSteuer:
		// valid
	default:
		return errors.New("unknown stammdaten section")
	}
	if c.FieldName == "" {
		return errors.New("field_name is required")
	}
	if c.OldValue == c.NewValue {
		return errors.New("old and new value are identical")
	}
	return nil
}

// StaffMasterDataChangeCreator is append-only; there is no application-side
// update or delete on the trail.
type StaffMasterDataChangeCreator interface {
	Create(ctx context.Context, event *StaffMasterDataChange) error
}

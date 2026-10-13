package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Gender values for users.staff_master_data.gender. Nil means "keine Angabe".
const (
	GenderFemale  = "female"
	GenderMale    = "male"
	GenderDiverse = "diverse"
)

// StaffMasterData is the 1:1 Stammdaten extension of a staff member (#1423):
// contact and contract fields kept off users.staff so the staff directory
// queries stay narrow. Name and birthday remain on users.persons; the
// employment type remains on users.staff — this table carries only what
// nothing else stores.
type StaffMasterData struct {
	Model `bun:"schema:users,table:staff_master_data"`
	TenantModel
	StaffID int64 `bun:"staff_id,notnull" json:"staff_id"`

	Gender *string `bun:"gender" json:"gender,omitempty"`

	AddressStreet         *string `bun:"address_street" json:"address_street,omitempty"`
	AddressPostalCode     *string `bun:"address_postal_code" json:"address_postal_code,omitempty"`
	AddressCity           *string `bun:"address_city" json:"address_city,omitempty"`
	Phone                 *string `bun:"phone" json:"phone,omitempty"`
	Email                 *string `bun:"email" json:"email,omitempty"`
	EmergencyContactName  *string `bun:"emergency_contact_name" json:"emergency_contact_name,omitempty"`
	EmergencyContactPhone *string `bun:"emergency_contact_phone" json:"emergency_contact_phone,omitempty"`

	EntryDate        *calendar.Date `bun:"entry_date,type:date" json:"entry_date,omitempty"`
	ContractEndDate  *calendar.Date `bun:"contract_end_date,type:date" json:"contract_end_date,omitempty"`
	ProbationEndDate *calendar.Date `bun:"probation_end_date,type:date" json:"probation_end_date,omitempty"`
	WeeklyHours      *float64       `bun:"weekly_hours" json:"weekly_hours,omitempty"`
}

// StaffQualification is one qualification row of a staff member (#1423):
// Erste-Hilfe, Schwimmschein, Fortbildungen — each with optional acquisition
// and expiry dates.
type StaffQualification struct {
	Model `bun:"schema:users,table:staff_qualifications"`
	TenantModel
	StaffID    int64          `bun:"staff_id,notnull" json:"staff_id"`
	Name       string         `bun:"name,notnull" json:"name"`
	AcquiredOn *calendar.Date `bun:"acquired_on,type:date" json:"acquired_on,omitempty"`
	ExpiresOn  *calendar.Date `bun:"expires_on,type:date" json:"expires_on,omitempty"`
}

// StaffFinancialData is the 1:1 bank & tax record of a staff member (#1423).
// Deliberately its own table: nothing outside the staff:financial code path
// may ever join or list these columns. Generic Staff JSON must not expose
// them; the financial handlers use dedicated masked/reveal responses.
type StaffFinancialData struct {
	Model `bun:"schema:users,table:staff_financial_data"`
	TenantModel
	StaffID              int64   `bun:"staff_id,notnull" json:"staff_id"`
	IBAN                 *string `bun:"iban" json:"-"`
	TaxID                *string `bun:"tax_id" json:"-"`
	SocialSecurityNumber *string `bun:"social_security_number" json:"-"`
}

// StaffMasterDataRepository persists the 1:1 master-data rows.
type StaffMasterDataRepository interface {
	Create(ctx context.Context, data *StaffMasterData) error
	Update(ctx context.Context, data *StaffMasterData) error
	FindByStaffID(ctx context.Context, staffID int64) (*StaffMasterData, error)
}

// StaffQualificationRepository persists the qualification rows.
type StaffQualificationRepository interface {
	ListByStaffID(ctx context.Context, staffID int64) ([]*StaffQualification, error)
	// ReplaceForStaff atomically replaces the staff member's qualification
	// list. A domain operation rather than a filter (backend rule 2): the
	// section edit modal submits the full list, and partial diffs would
	// leave concurrent editors with merged half-lists.
	ReplaceForStaff(ctx context.Context, staffID int64, qualifications []*StaffQualification) error
}

// StaffFinancialDataRepository persists the 1:1 financial rows.
type StaffFinancialDataRepository interface {
	Create(ctx context.Context, data *StaffFinancialData) error
	Update(ctx context.Context, data *StaffFinancialData) error
	FindByStaffID(ctx context.Context, staffID int64) (*StaffFinancialData, error)
}

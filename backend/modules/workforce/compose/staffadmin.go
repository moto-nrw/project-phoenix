package compose

import (
	"errors"
	"log/slog"
	"time"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/application"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

// StaffAdminDependencies are the collaborators the personnel-record
// administration cannot own: the staff and person rows behind a record, and the
// audit trail. Both are required, so an unaudited composition fails at startup
// instead of at the first change.
type StaffAdminDependencies struct {
	DB       *bun.DB
	Subjects workforce.StaffAdminSubjects
	Audit    workforce.StaffAdminAudit
	// Allows evaluates a required permission against a held set with the
	// shared wildcard rules; the root binds the security runtime's matcher.
	Allows  func(required string, held []string) bool
	Observe func(Observation)
	// Now is the clock access-log rows and cleanup delays are measured
	// against; nil means the wall clock. Tests pin it.
	Now    func() time.Time
	Logger *slog.Logger
	// BankData seals and opens the bank and tax section (BANK_DATA_KEYS, #6332).
	BankData workforce.BankDataCipher
}

// NewStaffAdmin composes the personnel-record administration over the
// Workforce-owned record rows.
func NewStaffAdmin(dependencies StaffAdminDependencies) (*workforce.StaffAdmin, error) {
	if dependencies.DB == nil || dependencies.Subjects == nil || dependencies.Audit == nil || dependencies.Allows == nil || dependencies.BankData == nil {
		return nil, errors.New("staff admin: database, staff subjects, audit trail, permission matcher and bank data cipher are required")
	}
	store := postgres.New(postgres.DatabaseRuntime(dependencies.DB))
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	logger := dependencies.Logger
	if logger == nil {
		logger = slog.Default()
	}
	unit := transaction{lock: store.AcquireXactLock}
	records := application.NewRecords(store, unit, application.Clock{Time: now}, application.NewOptionalObserver(dependencies.Observe))
	return workforce.NewStaffAdmin(application.NewStaffAdminEngine(application.NewStaffAdmin(
		records, unit, application.NewStaffSubjects(dependencies.Subjects), application.NewStaffAudit(dependencies.Audit),
		domain.DocumentAuthority{
			Health: permissions.StaffDocumentsHealth, Financial: permissions.StaffFinancial, General: permissions.StaffDocuments,
			Allows: dependencies.Allows,
		},
		application.Clock{Time: now}, logger.Warn, dependencies.BankData, postgres.NewTraegerStaffStore(),
	), records)), nil
}

// NewTraegerStaffRecords composes the Personalstamm of the Träger-Portal
// (#4705, ADR 0089): the Träger tables in the schema workforce, the school's
// employments and bank rows through the same staff subjects and audit trail
// as the personnel record, and the bank ring.
func NewTraegerStaffRecords(dependencies StaffAdminDependencies, collaborators workforce.TraegerStaffPorts) (workforce.TraegerStaffRecords, error) {
	if dependencies.Subjects == nil || dependencies.Audit == nil || dependencies.BankData == nil ||
		collaborators.Schools == nil || collaborators.Employments == nil || collaborators.Persons == nil || collaborators.Access == nil || collaborators.Runtime == nil {
		return nil, errors.New("traeger staff records: staff subjects, audit trail, bank data cipher, schools, employments, persons, access log and runtime are required")
	}
	return application.NewTraegerStaff(postgres.NewTraegerStaffStore(), collaborators,
		application.NewStaffSubjects(dependencies.Subjects), application.NewStaffAudit(dependencies.Audit), dependencies.BankData), nil
}

// NewTraegerStaffReleases composes the release „Personal beim Träger“ the
// moto team grants in the Operator portal (ADR 0089 section 2).
func NewTraegerStaffReleases() workforce.TraegerStaffReleases {
	return application.NewTraegerStaffReleases(postgres.NewTraegerStaffStore())
}

// --- documents --------------------------------------------------------------

// NewStaffBankExportFields opens the sealed staff bank and tax values for the return export of a school (#6412); only the administrative export CLI binds it, never HTTP.
func NewStaffBankExportFields(cipher workforce.BankDataCipher) workforce.StaffBankExportFields {
	return application.NewStaffBankExportFields(cipher)
}

// NewStaffBankDataRekey is the maintenance-only bank backfill/rekey (#6332), never HTTP.
func NewStaffBankDataRekey(cipher workforce.BankDataRekeyCipher) workforce.StaffBankDataRekey {
	return application.NewStaffBankDataRekey(postgres.NewStaffBankRekeyStore(), cipher)
}

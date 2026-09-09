package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

const (
	tableStaffMasterData           = "users.staff_master_data"
	tableStaffQualifications       = "users.staff_qualifications"
	tableStaffFinancialData        = "users.staff_financial_data"
	tableStaffDocuments            = "users.staff_documents"
	tableStaffDocumentFileCleanups = "users.staff_document_file_cleanup"

	aliasStaffMasterData           = "staff_master_data"
	aliasStaffQualification        = "staff_qualification"
	aliasStaffFinancialData        = "staff_financial_data"
	aliasStaffDocument             = "staff_document"
	aliasStaffDocumentFileCleanup  = "staff_document_file_cleanup"
	staffDocumentFileCleanupUnique = "CONFLICT (tenant_id, filename_stored) DO NOTHING"
)

type staffMasterDataRow struct {
	bun.BaseModel         `bun:"table:users.staff_master_data,alias:staff_master_data"`
	ID                    int64         `bun:"id,pk,autoincrement"`
	TenantID              int64         `bun:"tenant_id,notnull"`
	StaffID               int64         `bun:"staff_id,notnull"`
	Gender                *string       `bun:"gender"`
	AddressStreet         *string       `bun:"address_street"`
	AddressPostalCode     *string       `bun:"address_postal_code"`
	AddressCity           *string       `bun:"address_city"`
	Phone                 *string       `bun:"phone"`
	Email                 *string       `bun:"email"`
	EmergencyContactName  *string       `bun:"emergency_contact_name"`
	EmergencyContactPhone *string       `bun:"emergency_contact_phone"`
	EntryDate             *calendarDate `bun:"entry_date,type:date"`
	ContractEndDate       *calendarDate `bun:"contract_end_date,type:date"`
	ProbationEndDate      *calendarDate `bun:"probation_end_date,type:date"`
	WeeklyHours           *float64      `bun:"weekly_hours"`
	CreatedAt             time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt             time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type staffQualificationRow struct {
	bun.BaseModel `bun:"table:users.staff_qualifications,alias:staff_qualification"`
	ID            int64         `bun:"id,pk,autoincrement"`
	TenantID      int64         `bun:"tenant_id,notnull"`
	StaffID       int64         `bun:"staff_id,notnull"`
	Name          string        `bun:"name,notnull"`
	AcquiredOn    *calendarDate `bun:"acquired_on,type:date"`
	ExpiresOn     *calendarDate `bun:"expires_on,type:date"`
	CreatedAt     time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type staffFinancialDataRow struct {
	bun.BaseModel        `bun:"table:users.staff_financial_data,alias:staff_financial_data"`
	ID                   int64     `bun:"id,pk,autoincrement"`
	TenantID             int64     `bun:"tenant_id,notnull"`
	StaffID              int64     `bun:"staff_id,notnull"`
	IBAN                 *string   `bun:"iban"`
	TaxID                *string   `bun:"tax_id"`
	SocialSecurityNumber *string   `bun:"social_security_number"`
	CreatedAt            time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt            time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type staffDocumentRow struct {
	bun.BaseModel   `bun:"table:users.staff_documents,alias:staff_document"`
	ID              int64      `bun:"id,pk,autoincrement"`
	TenantID        int64      `bun:"tenant_id,notnull"`
	StaffID         int64      `bun:"staff_id,notnull"`
	Category        string     `bun:"category,notnull"`
	FilenameDisplay string     `bun:"filename_display,notnull"`
	FilenameStored  string     `bun:"filename_stored,notnull"`
	SizeBytes       int64      `bun:"size_bytes,notnull"`
	ContentType     string     `bun:"content_type,notnull"`
	UploadedBy      int64      `bun:"uploaded_by,notnull"`
	DeletedAt       *time.Time `bun:"deleted_at"`
	DeletedBy       *int64     `bun:"deleted_by"`
	FileDeletedAt   *time.Time `bun:"file_deleted_at"`
	CreatedAt       time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type staffDocumentFileCleanupRow struct {
	bun.BaseModel  `bun:"table:users.staff_document_file_cleanup,alias:staff_document_file_cleanup"`
	ID             int64      `bun:"id,pk,autoincrement"`
	TenantID       int64      `bun:"tenant_id,notnull"`
	StaffID        int64      `bun:"staff_id,notnull"`
	FilenameStored string     `bun:"filename_stored,notnull"`
	RetryAfter     time.Time  `bun:"retry_after,notnull"`
	CleanedAt      *time.Time `bun:"cleaned_at"`
	CreatedAt      time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// --- master data ---

func (s *Store) FindStaffMasterData(ctx context.Context, staffID int64) (domain.StaffMasterData, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffMasterData{}, false, domain.OperationStats{}, err
	}
	row := &staffMasterDataRow{}
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableStaffMasterData+` AS "staff_master_data"`).
		Where(`"staff_master_data".staff_id = ?`, staffID), aliasStaffMasterData, tenantID)
	found, stats, err := scanOne(ctx, query, "find staff master data")
	if err != nil || !found {
		return domain.StaffMasterData{}, found, stats, err
	}
	return staffMasterDataToDomain(*row), true, stats, nil
}

func (s *Store) CreateStaffMasterData(ctx context.Context, value domain.StaffMasterData) (domain.StaffMasterData, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffMasterData{}, domain.OperationStats{}, err
	}
	row := staffMasterDataFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableStaffMasterData).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StaffMasterData{}, stats, fmt.Errorf("workforce postgres: insert staff master data: %w", err)
	}
	stats.Rows = 1
	return staffMasterDataToDomain(*row), stats, nil
}

func (s *Store) UpdateStaffMasterData(ctx context.Context, value domain.StaffMasterData) (domain.StaffMasterData, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffMasterData{}, false, domain.OperationStats{}, err
	}
	row := staffMasterDataFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	query := withTenant(db.NewUpdate().Model(row).
		ModelTableExpr(tableStaffMasterData+` AS "staff_master_data"`).
		ExcludeColumn("created_at", "updated_at").
		Set("updated_at = NOW()").
		WherePK().
		Returning("*"), aliasStaffMasterData, tenantID)
	stats, err := execAffected(ctx, query, "update staff master data")
	if err != nil {
		return domain.StaffMasterData{}, false, stats, err
	}
	if stats.Rows != 1 {
		stats.Rows = 0
		return domain.StaffMasterData{}, false, stats, nil
	}
	return staffMasterDataToDomain(*row), true, stats, nil
}

// --- qualifications ---

func (s *Store) ListStaffQualifications(ctx context.Context, staffID int64) ([]domain.StaffQualification, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffQualificationRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(tableStaffQualifications+` AS "staff_qualification"`).
		Where(`"staff_qualification".staff_id = ?`, staffID), aliasStaffQualification, tenantID).
		OrderExpr(`"staff_qualification".id ASC`)
	stats, err := scanAll(ctx, query, "list staff qualifications")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	return staffQualificationsToDomain(rows), stats, nil
}

func (s *Store) DeleteStaffQualifications(ctx context.Context, staffID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*staffQualificationRow)(nil)).
		ModelTableExpr(tableStaffQualifications+` AS "staff_qualification"`).
		Where(`"staff_qualification".staff_id = ?`, staffID), aliasStaffQualification, tenantID)
	return execAffected(ctx, query, "delete staff qualifications")
}

func (s *Store) InsertStaffQualifications(ctx context.Context, values []domain.StaffQualification) ([]domain.StaffQualification, domain.OperationStats, error) {
	if len(values) == 0 {
		return []domain.StaffQualification{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := make([]staffQualificationRow, 0, len(values))
	for _, value := range values {
		row := staffQualificationFromDomain(value)
		if row.TenantID == 0 {
			row.TenantID = tenantID
		}
		rows = append(rows, *row)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(&rows).ModelTableExpr(tableStaffQualifications).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("workforce postgres: insert staff qualifications: %w", err)
	}
	stats.Rows = int64(len(rows))
	return staffQualificationsToDomain(rows), stats, nil
}

// --- financial data ---

func (s *Store) FindStaffFinancialData(ctx context.Context, staffID int64) (domain.StaffFinancialData, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffFinancialData{}, false, domain.OperationStats{}, err
	}
	row := &staffFinancialDataRow{}
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableStaffFinancialData+` AS "staff_financial_data"`).
		Where(`"staff_financial_data".staff_id = ?`, staffID), aliasStaffFinancialData, tenantID)
	found, stats, err := scanOne(ctx, query, "find staff financial data")
	if err != nil || !found {
		return domain.StaffFinancialData{}, found, stats, err
	}
	return staffFinancialDataToDomain(*row), true, stats, nil
}

func (s *Store) CreateStaffFinancialData(ctx context.Context, value domain.StaffFinancialData) (domain.StaffFinancialData, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffFinancialData{}, domain.OperationStats{}, err
	}
	row := staffFinancialDataFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableStaffFinancialData).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StaffFinancialData{}, stats, fmt.Errorf("workforce postgres: insert staff financial data: %w", err)
	}
	stats.Rows = 1
	return staffFinancialDataToDomain(*row), stats, nil
}

func (s *Store) UpdateStaffFinancialData(ctx context.Context, value domain.StaffFinancialData) (domain.StaffFinancialData, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffFinancialData{}, false, domain.OperationStats{}, err
	}
	row := staffFinancialDataFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	query := withTenant(db.NewUpdate().Model(row).
		ModelTableExpr(tableStaffFinancialData+` AS "staff_financial_data"`).
		ExcludeColumn("created_at", "updated_at").
		Set("updated_at = NOW()").
		WherePK().
		Returning("*"), aliasStaffFinancialData, tenantID)
	stats, err := execAffected(ctx, query, "update staff financial data")
	if err != nil {
		return domain.StaffFinancialData{}, false, stats, err
	}
	if stats.Rows != 1 {
		stats.Rows = 0
		return domain.StaffFinancialData{}, false, stats, nil
	}
	return staffFinancialDataToDomain(*row), true, stats, nil
}

// --- documents ---

func (s *Store) CreateStaffDocument(ctx context.Context, value domain.StaffDocument) (domain.StaffDocument, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffDocument{}, domain.OperationStats{}, err
	}
	row := staffDocumentFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableStaffDocuments).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StaffDocument{}, stats, fmt.Errorf("workforce postgres: insert staff document: %w", err)
	}
	stats.Rows = 1
	return staffDocumentToDomain(*row), stats, nil
}

func (s *Store) FindStaffDocument(ctx context.Context, staffID, documentID int64, includeDeleted bool) (domain.StaffDocument, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffDocument{}, false, domain.OperationStats{}, err
	}
	row := &staffDocumentRow{}
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableStaffDocuments+` AS "staff_document"`).
		Where(`"staff_document".id = ?`, documentID).
		Where(`"staff_document".staff_id = ?`, staffID), aliasStaffDocument, tenantID)
	if !includeDeleted {
		query = query.Where(`"staff_document".deleted_at IS NULL`)
	}
	found, stats, err := scanOne(ctx, query, "find staff document")
	if err != nil || !found {
		return domain.StaffDocument{}, found, stats, err
	}
	return staffDocumentToDomain(*row), true, stats, nil
}

func (s *Store) ListStaffDocuments(ctx context.Context, filter domain.StaffDocumentFilter) ([]domain.StaffDocument, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffDocumentRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(tableStaffDocuments+` AS "staff_document"`), aliasStaffDocument, tenantID)
	if filter.StaffID > 0 {
		query = query.Where(`"staff_document".staff_id = ?`, filter.StaffID)
	}
	if filter.StaffIDs != nil {
		if len(filter.StaffIDs) == 0 {
			query = query.Where("FALSE")
		} else {
			query = query.Where(`"staff_document".staff_id IN (?)`, bun.List(filter.StaffIDs))
		}
	}
	if filter.Categories != nil {
		if len(filter.Categories) == 0 {
			query = query.Where("FALSE")
		} else {
			query = query.Where(`"staff_document".category IN (?)`, bun.List(filter.Categories))
		}
	}
	switch {
	case filter.DeletedOnly:
		query = query.Where(`"staff_document".deleted_at IS NOT NULL`)
	case !filter.IncludeDeleted:
		query = query.Where(`"staff_document".deleted_at IS NULL`)
	}
	if filter.FilePending {
		query = query.Where(`"staff_document".file_deleted_at IS NULL`)
	}
	query = query.OrderExpr(`"staff_document".created_at DESC, "staff_document".id DESC`)
	stats, err := scanAll(ctx, query, "list staff documents")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.StaffDocument, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffDocumentToDomain(row))
	}
	return result, stats, nil
}

func (s *Store) SoftDeleteStaffDocument(ctx context.Context, id, deletedBy int64, at time.Time) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*staffDocumentRow)(nil)).
		ModelTableExpr(tableStaffDocuments+` AS "staff_document"`).
		Set("deleted_at = ?", at).
		Set("deleted_by = ?", deletedBy).
		Set("updated_at = NOW()").
		Where(`"staff_document".id = ?`, id).
		Where(`"staff_document".deleted_at IS NULL`), aliasStaffDocument, tenantID)
	stats, err := execAffected(ctx, query, "soft delete staff document")
	return stats.Rows, stats, err
}

func (s *Store) MarkStaffDocumentFileDeleted(ctx context.Context, id int64, at time.Time) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*staffDocumentRow)(nil)).
		ModelTableExpr(tableStaffDocuments+` AS "staff_document"`).
		Set("file_deleted_at = ?", at).
		Set("updated_at = NOW()").
		Where(`"staff_document".id = ?`, id).
		Where(`"staff_document".file_deleted_at IS NULL`), aliasStaffDocument, tenantID)
	return execAffected(ctx, query, "mark staff document file deleted")
}

// --- file cleanup intents ---

func (s *Store) QueueStaffDocumentFileCleanup(ctx context.Context, value domain.StaffDocumentFileCleanup) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := &staffDocumentFileCleanupRow{
		TenantID: value.TenantID, StaffID: value.StaffID, FilenameStored: value.FilenameStored,
		RetryAfter: value.RetryAfter, CleanedAt: value.CleanedAt,
	}
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	query := db.NewInsert().Model(row).ModelTableExpr(tableStaffDocumentFileCleanups).On(staffDocumentFileCleanupUnique)
	return execAffected(ctx, query, "queue staff document file cleanup")
}

// ListQueuedStaffDocumentFileCleanups locks the eligible rows for the caller's
// transaction. FOR UPDATE SKIP LOCKED is load-bearing: an upload whose
// metadata transaction has already stamped cleaned_at but not yet committed
// holds that row lock, and skipping it keeps this pass from deleting the bytes
// of a document that is about to exist.
func (s *Store) ListQueuedStaffDocumentFileCleanups(ctx context.Context, staffID int64, now time.Time) ([]domain.StaffDocumentFileCleanup, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffDocumentFileCleanupRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(tableStaffDocumentFileCleanups+` AS "staff_document_file_cleanup"`).
		Where(`"staff_document_file_cleanup".retry_after <= ?`, now).
		Where(`"staff_document_file_cleanup".cleaned_at IS NULL`), aliasStaffDocumentFileCleanup, tenantID)
	if staffID > 0 {
		query = query.Where(`"staff_document_file_cleanup".staff_id = ?`, staffID)
	}
	query = query.OrderExpr(`"staff_document_file_cleanup".id ASC`).For("UPDATE SKIP LOCKED")
	stats, err := scanAll(ctx, query, "list queued staff document file cleanups")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.StaffDocumentFileCleanup, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffDocumentFileCleanupToDomain(row))
	}
	return result, stats, nil
}

func (s *Store) CompleteStaffDocumentFileCleanup(ctx context.Context, id int64, at time.Time) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*staffDocumentFileCleanupRow)(nil)).
		ModelTableExpr(tableStaffDocumentFileCleanups+` AS "staff_document_file_cleanup"`).
		Set("cleaned_at = ?", at).
		Set("updated_at = NOW()").
		Where(`"staff_document_file_cleanup".id = ?`, id).
		Where(`"staff_document_file_cleanup".cleaned_at IS NULL`), aliasStaffDocumentFileCleanup, tenantID)
	return execAffected(ctx, query, "complete staff document file cleanup")
}

func (s *Store) CompleteStaffDocumentFileCleanupByFilename(ctx context.Context, filename string, at time.Time) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*staffDocumentFileCleanupRow)(nil)).
		ModelTableExpr(tableStaffDocumentFileCleanups+` AS "staff_document_file_cleanup"`).
		Set("cleaned_at = ?", at).
		Set("updated_at = NOW()").
		Where(`"staff_document_file_cleanup".filename_stored = ?`, filename).
		Where(`"staff_document_file_cleanup".cleaned_at IS NULL`), aliasStaffDocumentFileCleanup, tenantID)
	return execAffected(ctx, query, "complete staff document file cleanup by filename")
}

func (s *Store) ActivateStaffDocumentFileCleanup(ctx context.Context, filename string, at time.Time) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*staffDocumentFileCleanupRow)(nil)).
		ModelTableExpr(tableStaffDocumentFileCleanups+` AS "staff_document_file_cleanup"`).
		Set("retry_after = ?", at).
		Set("updated_at = NOW()").
		Where(`"staff_document_file_cleanup".filename_stored = ?`, filename).
		Where(`"staff_document_file_cleanup".cleaned_at IS NULL`), aliasStaffDocumentFileCleanup, tenantID)
	return execAffected(ctx, query, "activate staff document file cleanup")
}

// --- mapping ---

func staffMasterDataFromDomain(value domain.StaffMasterData) *staffMasterDataRow {
	return &staffMasterDataRow{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Gender: value.Gender,
		AddressStreet: value.AddressStreet, AddressPostalCode: value.AddressPostalCode, AddressCity: value.AddressCity,
		Phone: value.Phone, Email: value.Email, EmergencyContactName: value.EmergencyContactName,
		EmergencyContactPhone: value.EmergencyContactPhone, EntryDate: optionalCalendarDate(value.EntryDate),
		ContractEndDate: optionalCalendarDate(value.ContractEndDate), ProbationEndDate: optionalCalendarDate(value.ProbationEndDate),
		WeeklyHours: value.WeeklyHours, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func staffMasterDataToDomain(row staffMasterDataRow) domain.StaffMasterData {
	return domain.StaffMasterData{
		ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID, Gender: row.Gender,
		AddressStreet: row.AddressStreet, AddressPostalCode: row.AddressPostalCode, AddressCity: row.AddressCity,
		Phone: row.Phone, Email: row.Email, EmergencyContactName: row.EmergencyContactName,
		EmergencyContactPhone: row.EmergencyContactPhone, EntryDate: calendarDateString(row.EntryDate),
		ContractEndDate: calendarDateString(row.ContractEndDate), ProbationEndDate: calendarDateString(row.ProbationEndDate),
		WeeklyHours: row.WeeklyHours, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func staffQualificationFromDomain(value domain.StaffQualification) *staffQualificationRow {
	return &staffQualificationRow{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Name: value.Name,
		AcquiredOn: optionalCalendarDate(value.AcquiredOn), ExpiresOn: optionalCalendarDate(value.ExpiresOn),
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func staffQualificationsToDomain(rows []staffQualificationRow) []domain.StaffQualification {
	result := make([]domain.StaffQualification, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.StaffQualification{
			ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID, Name: row.Name,
			AcquiredOn: calendarDateString(row.AcquiredOn), ExpiresOn: calendarDateString(row.ExpiresOn),
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return result
}

func staffFinancialDataFromDomain(value domain.StaffFinancialData) *staffFinancialDataRow {
	return &staffFinancialDataRow{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, IBAN: value.IBAN, TaxID: value.TaxID,
		SocialSecurityNumber: value.SocialSecurityNumber, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func staffFinancialDataToDomain(row staffFinancialDataRow) domain.StaffFinancialData {
	return domain.StaffFinancialData{
		ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID, IBAN: row.IBAN, TaxID: row.TaxID,
		SocialSecurityNumber: row.SocialSecurityNumber, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func staffDocumentFromDomain(value domain.StaffDocument) *staffDocumentRow {
	return &staffDocumentRow{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Category: value.Category,
		FilenameDisplay: value.FilenameDisplay, FilenameStored: value.FilenameStored, SizeBytes: value.SizeBytes,
		ContentType: value.ContentType, UploadedBy: value.UploadedBy, DeletedAt: value.DeletedAt, DeletedBy: value.DeletedBy,
		FileDeletedAt: value.FileDeletedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func staffDocumentToDomain(row staffDocumentRow) domain.StaffDocument {
	return domain.StaffDocument{
		ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID, Category: row.Category,
		FilenameDisplay: row.FilenameDisplay, FilenameStored: row.FilenameStored, SizeBytes: row.SizeBytes,
		ContentType: row.ContentType, UploadedBy: row.UploadedBy, DeletedAt: row.DeletedAt, DeletedBy: row.DeletedBy,
		FileDeletedAt: row.FileDeletedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func staffDocumentFileCleanupToDomain(row staffDocumentFileCleanupRow) domain.StaffDocumentFileCleanup {
	return domain.StaffDocumentFileCleanup{
		ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID, FilenameStored: row.FilenameStored,
		RetryAfter: row.RetryAfter, CleanedAt: row.CleanedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

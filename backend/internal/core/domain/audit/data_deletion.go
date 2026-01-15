package audit

import (
	"errors"
	"time"
)

// DataDeletion represents a record of deleted data for GDPR compliance
type DataDeletion struct {
	ID             int64                  `bun:"id,pk,autoincrement" json:"id"`
	StudentID      int64                  `bun:"student_id,notnull" json:"student_id"`
	DeletionType   string                 `bun:"deletion_type,notnull" json:"deletion_type"` // 'visit_retention', 'manual', 'gdpr_request'
	RecordsDeleted int                    `bun:"records_deleted,notnull" json:"records_deleted"`
	DeletionReason string                 `bun:"deletion_reason" json:"deletion_reason,omitempty"`
	DeletedBy      string                 `bun:"deleted_by,notnull" json:"deleted_by"` // 'system' or account username
	DeletedAt      time.Time              `bun:"deleted_at,notnull,default:now()" json:"deleted_at"`
	Metadata       map[string]interface{} `bun:"metadata,type:jsonb" json:"metadata,omitempty"`
}

// DeletionType constants
const (
	DeletionTypeVisitRetention = "visit_retention"
	DeletionTypeManual         = "manual"
	DeletionTypeGDPRRequest    = "gdpr_request"
)

// TableName returns the database table name
func (dd *DataDeletion) TableName() string {
	return "audit.data_deletions"
}

// Validate ensures data deletion record is valid
func (dd *DataDeletion) Validate() error {
	if dd.StudentID <= 0 {
		return errors.New("student ID is required")
	}

	if dd.DeletionType == "" {
		return errors.New("deletion type is required")
	}

	// Validate deletion type
	switch dd.DeletionType {
	case DeletionTypeVisitRetention, DeletionTypeManual, DeletionTypeGDPRRequest:
		// Valid types
	default:
		return errors.New("invalid deletion type")
	}

	if dd.RecordsDeleted < 0 {
		return errors.New("records deleted cannot be negative")
	}

	if dd.DeletedBy == "" {
		return errors.New("deleted by is required")
	}

	if dd.DeletedAt.IsZero() {
		dd.DeletedAt = time.Now()
	}

	return nil
}

// GetID implements the base.Entity interface
func (dd *DataDeletion) GetID() interface{} {
	return dd.ID
}

// GetCreatedAt implements the base.Entity interface
func (dd *DataDeletion) GetCreatedAt() time.Time {
	return dd.DeletedAt
}

// GetUpdatedAt implements the base.Entity interface
func (dd *DataDeletion) GetUpdatedAt() time.Time {
	return dd.DeletedAt
}

// GetMetadata returns the metadata map
func (dd *DataDeletion) GetMetadata() map[string]interface{} {
	if dd.Metadata == nil {
		dd.Metadata = make(map[string]interface{})
	}
	return dd.Metadata
}

// SetMetadata sets metadata information
func (dd *DataDeletion) SetMetadata(key string, value interface{}) {
	if dd.Metadata == nil {
		dd.Metadata = make(map[string]interface{})
	}
	dd.Metadata[key] = value
}

// NewDataDeletion creates a new data deletion record
func NewDataDeletion(studentID int64, deletionType string, recordsDeleted int, deletedBy string) *DataDeletion {
	return &DataDeletion{
		StudentID:      studentID,
		DeletionType:   deletionType,
		RecordsDeleted: recordsDeleted,
		DeletedBy:      deletedBy,
		DeletedAt:      time.Now(),
		Metadata:       make(map[string]interface{}),
	}
}

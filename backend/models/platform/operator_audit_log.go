package platform

import (
	"encoding/json"
	"net"
	"time"

	"github.com/uptrace/bun"
)

// tablePlatformOperatorAuditLog is the schema-qualified table name
const tablePlatformOperatorAuditLog = "platform.operator_audit_log"

// Common audit action constants
const (
	ActionCreate        = "create"
	ActionUpdate        = "update"
	ActionDelete        = "delete"
	ActionStatusChange  = "status_change"
	ActionPublish       = "publish"
	ActionLogin         = "login"
	ActionAddComment    = "add_comment"
	ActionDeleteComment = "delete_comment"
)

// Common resource type constants
const (
	ResourceAnnouncement = "announcement"
	ResourceSuggestion   = "suggestion"
	ResourceComment      = "operator_comment"
	ResourceOperator     = "operator"
)

// OperatorAuditLog tracks operator actions for auditing
type OperatorAuditLog struct {
	ID           int64           `bun:"id,pk,autoincrement" json:"id"`
	OperatorID   int64           `bun:"operator_id,notnull" json:"operator_id"`
	Action       string          `bun:"action,notnull" json:"action"`
	ResourceType string          `bun:"resource_type,notnull" json:"resource_type"`
	ResourceID   *int64          `bun:"resource_id" json:"resource_id,omitempty"`
	Changes      json.RawMessage `bun:"changes,type:jsonb" json:"changes,omitempty"`
	RequestIP    net.IP          `bun:"request_ip,type:inet" json:"request_ip,omitempty"`
	CreatedAt    time.Time       `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`

	// Relations
	Operator *Operator `bun:"rel:belongs-to,join:operator_id=id" json:"operator,omitempty"`
}

func (l *OperatorAuditLog) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tablePlatformOperatorAuditLog)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tablePlatformOperatorAuditLog)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tablePlatformOperatorAuditLog)
	}
	return nil
}

// TableName returns the database table name
func (l *OperatorAuditLog) TableName() string {
	return tablePlatformOperatorAuditLog
}

// SetChanges sets the changes field from a map
func (l *OperatorAuditLog) SetChanges(changes map[string]any) error {
	data, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	l.Changes = data
	return nil
}

// GetChanges parses the changes field into a map
func (l *OperatorAuditLog) GetChanges() (map[string]any, error) {
	if l.Changes == nil {
		return nil, nil
	}
	var changes map[string]any
	if err := json.Unmarshal(l.Changes, &changes); err != nil {
		return nil, err
	}
	return changes, nil
}

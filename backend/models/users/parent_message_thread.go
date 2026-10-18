package users

// Sender kinds shared by threads and messages. A message originates either
// from a guardian (parents portal) or from staff (tenant portal).
const (
	ParentMessageSenderGuardian = "guardian"
	ParentMessageSenderStaff    = "staff"
	ParentMessageSenderSystem   = "system"
)

type MessageableGuardian struct {
	AccountID        int64  `bun:"account_id" json:"account_id"`
	Name             string `bun:"name" json:"name"`
	RelationshipType string `bun:"relationship_type" json:"relationship_type"`
	IsPrimary        bool   `bun:"is_primary" json:"is_primary"`
	PortalLocale     string `bun:"portal_locale" json:"-"`
}

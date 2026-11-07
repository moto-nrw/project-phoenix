package records

import (
	"errors"
	"time"
)

// TraegerDocumentDownload is one download of a shared document by the
// Träger-Büro in the Audit-owned audit.traeger_document_downloads ledger
// (#4692, ADR 0157): who, which document, when and from which client; never
// the title or the file name. A Träger is no tenant, so the entry carries its
// Träger and no school.
type TraegerDocumentDownload struct {
	ID             int64     `bun:"id,pk,autoincrement" json:"id"`
	OrganizationID int64     `bun:"organization_id,notnull" json:"organization_id"`
	DocumentID     int64     `bun:"document_id,notnull" json:"document_id"`
	ActorAccountID int64     `bun:"actor_account_id,notnull" json:"actor_account_id"`
	ClientIP       string    `bun:"client_ip,nullzero" json:"client_ip,omitempty"`
	UserAgent      string    `bun:"user_agent,nullzero" json:"user_agent,omitempty"`
	DownloadedAt   time.Time `bun:"downloaded_at,notnull,default:current_timestamp" json:"downloaded_at"`
}

// PlatformScoped marks the entry as tenantless: the Audit adapter appends it
// in the caller's Träger transaction without the tenant handshake.
func (*TraegerDocumentDownload) PlatformScoped() bool { return true }

// Validate requires the Träger, the document and the acting account.
func (d *TraegerDocumentDownload) Validate() error {
	switch {
	case d.OrganizationID <= 0:
		return errors.New("the Träger document download requires the Träger")
	case d.DocumentID <= 0:
		return errors.New("the Träger document download requires the document")
	case d.ActorAccountID <= 0:
		return errors.New("the Träger document download requires the acting account")
	}
	return nil
}

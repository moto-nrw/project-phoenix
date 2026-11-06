package organizationtenancy

import (
	"context"
	"errors"
	"time"
)

// Shared documents of a Träger (#4692, spec #4611 user stories 98 and 99):
// the Träger-Büro uploads a file it already has (a letter, a house rule, a
// list) and releases it for the parents of all its schools, of one school or
// of one group, or for its schools: all or one. A release is revoked, never
// deleted, and the revocation ends the access at once. The documents belong
// to the Träger like its master data (ADR 0105): the Träger-Büro reads and
// writes them in the Träger transaction. A parent reads them in the Träger
// transaction the parents portal opens, with the schools and groups its
// relationships open; a school reads them in its own tenant transaction,
// where the database answers only for the releases that cover it. Every
// opening by a parent or a school is recorded with the account.

// TraegerDocumentMaxBytes is the largest file a Träger shares.
const TraegerDocumentMaxBytes = 10 << 20 // the limit of the domain check and the table

// TraegerDocumentTitleMaxLength is the longest title, in characters.
const TraegerDocumentTitleMaxLength = 120

// TraegerDocumentAudience is whom a release is for.
type TraegerDocumentAudience string

const (
	// TraegerDocumentForParents releases a document for the parents of all
	// schools, of one school or of one group of a school.
	TraegerDocumentForParents TraegerDocumentAudience = "parents"
	// TraegerDocumentForSchools releases a document for all schools or for
	// one school of the Träger.
	TraegerDocumentForSchools TraegerDocumentAudience = "schools"
)

var (
	// ErrTraegerDocumentNotFound reports a document that is not the
	// Träger's, or one the reader's releases do not cover.
	ErrTraegerDocumentNotFound = errors.New("the Träger document was not found")
	// ErrTraegerDocumentReleaseNotFound reports a release that is not the
	// document's or is already revoked.
	ErrTraegerDocumentReleaseNotFound = errors.New("the Träger document release was not found")
	// ErrTraegerDocumentFileType reports a file that is not a PDF, Word,
	// Excel, PowerPoint, PNG or JPEG file by its name and its content.
	ErrTraegerDocumentFileType = errors.New("the file type is not admitted")
	// ErrTraegerDocumentTooLarge reports a file over TraegerDocumentMaxBytes.
	ErrTraegerDocumentTooLarge = errors.New("the file exceeds 10 MB")
	// ErrTraegerDocumentFileMissing reports an upload without a file.
	ErrTraegerDocumentFileMissing = errors.New("no file uploaded")
	// ErrTraegerDocumentAudienceInvalid reports a release whose school is
	// not the Träger's, whose group is not the school's, or whose shape the
	// audience does not admit.
	ErrTraegerDocumentAudienceInvalid = errors.New("the release names no audience of the Träger")
	// ErrTraegerDocumentAlreadyReleased reports a release that is already
	// active for the same audience.
	ErrTraegerDocumentAlreadyReleased = errors.New("the document is already released for this audience")
)

// TraegerDocumentRelease is one active release of a document. SchoolID nil
// means every school of the Träger; GroupID names a group of that school,
// GroupName its name when it was released.
type TraegerDocumentRelease struct {
	ID         int64                   `json:"id"`
	Audience   TraegerDocumentAudience `json:"audience"`
	SchoolID   *int64                  `json:"school_id"`
	GroupID    *int64                  `json:"group_id"`
	GroupName  string                  `json:"group_name"`
	ReleasedAt time.Time               `json:"released_at"`
}

// TraegerDocument is a document of the Träger with its active releases and
// how often parents and schools opened it.
type TraegerDocument struct {
	ID          int64                    `json:"id"`
	Title       string                   `json:"title"`
	FileName    string                   `json:"file_name"`
	ContentType string                   `json:"content_type"`
	SizeBytes   int                      `json:"size_bytes"`
	CreatedAt   time.Time                `json:"created_at"`
	Releases    []TraegerDocumentRelease `json:"releases"`
	Reads       int                      `json:"reads"`
}

// TraegerDocumentUpload is a file the Träger-Büro shares, as uploaded.
type TraegerDocumentUpload struct {
	Title    string
	FileName string
	Data     []byte
}

// TraegerDocumentReleaseInput names whom a document is released for.
type TraegerDocumentReleaseInput struct {
	Audience TraegerDocumentAudience `json:"audience"`
	SchoolID *int64                  `json:"school_id"`
	GroupID  *int64                  `json:"group_id"`
}

// TraegerDocumentFile is the stored file of a document.
type TraegerDocumentFile struct {
	FileName    string
	ContentType string
	Data        []byte
}

// TraegerDocumentGroup is a group of a school a release may name.
type TraegerDocumentGroup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// TraegerDocumentSchool is a school of the Träger with its groups.
type TraegerDocumentSchool struct {
	ID     int64                  `json:"id"`
	Name   string                 `json:"name"`
	Groups []TraegerDocumentGroup `json:"groups"`
}

// TraegerDocuments lets the Träger-Büro share documents. Every operation
// opens the transactions it needs itself: the Träger transaction of
// organizationID and, for the groups, the tenant transaction of a school,
// never both at once. accountID is the acting account the rows record.
type TraegerDocuments interface {
	// ListDocuments lists the documents, the newest first.
	ListDocuments(ctx context.Context, organizationID int64) ([]TraegerDocument, error)
	// Upload checks the title and the file and stores the document.
	Upload(ctx context.Context, organizationID, accountID int64, upload TraegerDocumentUpload) (TraegerDocument, error)
	// File reads the stored file of a document of the Träger.
	// File reads the file for a member of the Träger-Büro and writes the
	// download into the access log of each school the document reaches.
	File(ctx context.Context, organizationID, accountID, documentID int64) (TraegerDocumentFile, error)
	// Audiences lists the schools of the Träger with their groups.
	Audiences(ctx context.Context, organizationID int64) ([]TraegerDocumentSchool, error)
	// Release releases a document for an audience.
	Release(ctx context.Context, organizationID, accountID, documentID int64, input TraegerDocumentReleaseInput) (TraegerDocumentRelease, error)
	// Revoke revokes an active release of a document.
	Revoke(ctx context.Context, organizationID, accountID, documentID, releaseID int64) error
}

// TraegerDocumentPorts are what the Träger-Büro's documents need outside
// the owner: the schools of the Träger, the groups of a school and the
// transactions.
type TraegerDocumentPorts struct {
	Schools TraegerDocumentSchools
	Groups  TraegerDocumentGroups
	Runtime TraegerDocumentRuntime
	Access  TraegerDocumentAccessLog
}

// TraegerDocumentAccess is a download of the Träger-Büro.
type TraegerDocumentAccess struct {
	OrganizationID int64
	AccountID      int64
	DocumentID     int64
}

// TraegerDocumentAccessLog appends a download to the access log of the
// school whose tenant transaction the caller holds (ADR 0084 section 7).
type TraegerDocumentAccessLog interface {
	RecordAccess(ctx context.Context, access TraegerDocumentAccess) error
}

// TraegerDocumentSchools lists the undeleted schools of a Träger, without
// groups, outside any transaction.
type TraegerDocumentSchools interface {
	TraegerSchools(ctx context.Context, organizationID int64) ([]TraegerDocumentSchool, error)
}

// TraegerDocumentGroups lists the groups of the school of the caller's
// tenant transaction.
type TraegerDocumentGroups interface {
	SchoolGroups(ctx context.Context) ([]TraegerDocumentGroup, error)
}

// TraegerDocumentRuntime opens a school's tenant transaction or the Träger
// transaction.
type TraegerDocumentRuntime interface {
	WithinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error
	WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error
}

// TraegerDocumentParent is a parent account with what its relationships
// open at the Träger: per school of the Träger, the groups of its children
// there. A school without groups still opens the releases for all schools
// and for that school.
type TraegerDocumentParent struct {
	AccountID int64
	Schools   map[int64][]int64
}

// SharedDocument is a document as a parent or a school sees it: what it
// is and whom the Träger released it for.
type SharedDocument struct {
	ID          int64                    `json:"id"`
	Title       string                   `json:"title"`
	FileName    string                   `json:"file_name"`
	ContentType string                   `json:"content_type"`
	SizeBytes   int                      `json:"size_bytes"`
	ReleasedAt  time.Time                `json:"released_at"`
	Releases    []TraegerDocumentRelease `json:"releases"`
}

// ParentTraegerDocuments serves the documents a Träger released for parents.
// Both operations join the caller's Träger transaction of organizationID.
type ParentTraegerDocuments interface {
	// ParentDocuments lists the documents an active parent release covers
	// for the parent, the newest release first.
	ParentDocuments(ctx context.Context, organizationID int64, parent TraegerDocumentParent) ([]SharedDocument, error)
	// OpenParentDocument reads the file of such a document and records the
	// read; ErrTraegerDocumentNotFound for any other.
	OpenParentDocument(ctx context.Context, organizationID, documentID int64, parent TraegerDocumentParent) (TraegerDocumentFile, error)
}

// SchoolTraegerDocuments serves a school the documents its Träger released
// for it or for the parents at it. Both operations join the caller's tenant
// transaction of schoolID.
type SchoolTraegerDocuments interface {
	SchoolDocuments(ctx context.Context, schoolID int64) ([]SharedDocument, error)
	// OpenSchoolDocument reads the file of such a document and records the
	// read with the account; ErrTraegerDocumentNotFound for any other.
	OpenSchoolDocument(ctx context.Context, schoolID, accountID, documentID int64) (TraegerDocumentFile, error)
}

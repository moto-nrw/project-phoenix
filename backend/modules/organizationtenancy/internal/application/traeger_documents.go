package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
)

// Shared documents of a Träger (#4692). The Träger-Büro's service opens its
// transactions itself: a release for a group first reads the groups of the
// school in that school's tenant transaction, then writes in the Träger
// transaction, because the two never nest (ADR 0105). The parents and the
// schools join the transaction their caller opened.

// TraegerDocuments implements organizationtenancy.TraegerDocuments.
type TraegerDocuments struct {
	store ports.TraegerDocumentStore
	ports organizationtenancy.TraegerDocumentPorts
}

var _ organizationtenancy.TraegerDocuments = (*TraegerDocuments)(nil)

// NewTraegerDocuments builds the Träger-Büro's service.
func NewTraegerDocuments(store ports.TraegerDocumentStore, dependencies organizationtenancy.TraegerDocumentPorts) *TraegerDocuments {
	if store == nil || dependencies.Schools == nil || dependencies.Groups == nil || dependencies.Runtime == nil ||
		dependencies.Access == nil || dependencies.Downloads == nil {
		panic("organization tenancy: the document store, the schools, the groups, the runtime and both audit logs are required")
	}
	return &TraegerDocuments{store: store, ports: dependencies}
}

// ListDocuments lists the documents of the Träger.
func (d *TraegerDocuments) ListDocuments(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerDocument, error) {
	if organizationID <= 0 {
		return nil, errTraegerRequired
	}
	var documents []organizationtenancy.TraegerDocument
	err := d.ports.Runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var listErr error
		documents, listErr = d.store.ListDocuments(txCtx, organizationID)
		return listErr
	})
	return documents, err
}

// Upload checks the title and the file and stores the document.
func (d *TraegerDocuments) Upload(ctx context.Context, organizationID, accountID int64, upload organizationtenancy.TraegerDocumentUpload) (organizationtenancy.TraegerDocument, error) {
	if organizationID <= 0 {
		return organizationtenancy.TraegerDocument{}, errTraegerRequired
	}
	title := strings.TrimSpace(upload.Title)
	fields := validation.Errors{}
	requiredText(fields, "title", title, organizationtenancy.TraegerDocumentTitleMaxLength)
	if err := fields.Filter(); err != nil {
		return organizationtenancy.TraegerDocument{}, err
	}
	fileName := domain.TraegerDocumentFileName(upload.FileName)
	contentType, err := domain.TraegerDocumentContentType(fileName, upload.Data)
	if err != nil {
		return organizationtenancy.TraegerDocument{}, documentFileError(err)
	}
	var document organizationtenancy.TraegerDocument
	err = d.ports.Runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var insertErr error
		document, insertErr = d.store.InsertDocument(txCtx, organizationID, accountID, domain.TraegerDocumentWrite{
			Title: title, FileName: fileName, ContentType: contentType, Data: upload.Data,
		})
		return insertErr
	})
	return document, err
}

// documentFileError maps the domain's refusal of a file onto the contract.
func documentFileError(err error) error {
	switch {
	case errors.Is(err, domain.ErrDocumentFileMissing):
		return organizationtenancy.ErrTraegerDocumentFileMissing
	case errors.Is(err, domain.ErrDocumentTooLarge):
		return organizationtenancy.ErrTraegerDocumentTooLarge
	case errors.Is(err, domain.ErrDocumentFileType):
		return organizationtenancy.ErrTraegerDocumentFileType
	}
	return err
}

// File reads the stored file of a document of the Träger. The download lands
// in the Träger's ledger in the same Träger transaction, released or not
// (ADR 0157); after it, each school the document reaches through an active
// release and that still belongs to the Träger gets an entry in its access
// log (ADR 0084 section 7). A failed entry fails the download: no file leaves
// without its audit.
func (d *TraegerDocuments) File(ctx context.Context, download organizationtenancy.TraegerDocumentDownload) (organizationtenancy.TraegerDocumentFile, error) {
	if download.OrganizationID <= 0 {
		return organizationtenancy.TraegerDocumentFile{}, errTraegerRequired
	}
	var file organizationtenancy.TraegerDocumentFile
	var reached []int64
	var everySchool bool
	err := d.ports.Runtime.WithinTraeger(ctx, download.OrganizationID, func(txCtx context.Context) error {
		var readErr error
		file, reached, everySchool, readErr = d.readForDownload(txCtx, download)
		if readErr != nil {
			return readErr
		}
		return d.ports.Downloads.RecordDownload(txCtx, download)
	})
	if err != nil {
		return organizationtenancy.TraegerDocumentFile{}, err
	}
	schools, err := d.reachedSchools(ctx, download.OrganizationID, reached, everySchool)
	if err != nil {
		return organizationtenancy.TraegerDocumentFile{}, err
	}
	access := organizationtenancy.TraegerDocumentAccess{
		OrganizationID: download.OrganizationID, AccountID: download.AccountID, DocumentID: download.DocumentID,
	}
	if err := d.logSchools(ctx, access, schools); err != nil {
		return organizationtenancy.TraegerDocumentFile{}, err
	}
	return file, nil
}

// readForDownload reads the file and the schools its active releases name.
func (d *TraegerDocuments) readForDownload(ctx context.Context, download organizationtenancy.TraegerDocumentDownload) (organizationtenancy.TraegerDocumentFile, []int64, bool, error) {
	file, ok, err := d.store.DocumentFile(ctx, download.OrganizationID, download.DocumentID)
	if err == nil && !ok {
		err = organizationtenancy.ErrTraegerDocumentNotFound
	}
	if err != nil {
		return organizationtenancy.TraegerDocumentFile{}, nil, false, err
	}
	reached, everySchool, err := d.store.ReachedSchools(ctx, download.OrganizationID, download.DocumentID)
	return file, reached, everySchool, err
}

// reachedSchools are the schools whose access log gets a download: those an
// active release names, or all for a release for every school, kept only
// while they still belong to the Träger. A release keeps the ID of a school
// that since moved to another Träger or was erased; such a school never gets
// the entry. A document no release reaches logs at no school.
func (d *TraegerDocuments) reachedSchools(ctx context.Context, organizationID int64, reached []int64, everySchool bool) ([]int64, error) {
	if !everySchool && len(reached) == 0 {
		return nil, nil
	}
	schools, err := d.ports.Schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("organization tenancy: list schools of the Träger: %w", err)
	}
	current := make([]int64, 0, len(schools))
	for _, school := range schools {
		current = append(current, school.ID)
	}
	if everySchool {
		return current, nil
	}
	return slices.DeleteFunc(slices.Clone(reached), func(school int64) bool {
		return !slices.Contains(current, school)
	}), nil
}

// logSchools writes one entry per school into that school's access log, in
// the school's own tenant transaction, after the Träger transaction ended.
func (d *TraegerDocuments) logSchools(ctx context.Context, access organizationtenancy.TraegerDocumentAccess, schools []int64) error {
	sorted := slices.Clone(schools)
	slices.Sort(sorted)
	for _, school := range slices.Compact(sorted) {
		err := d.ports.Runtime.WithinSchool(ctx, school, func(txCtx context.Context) error {
			return d.ports.Access.RecordAccess(txCtx, access)
		})
		if err != nil {
			return fmt.Errorf("organization tenancy: access log of school %d: %w", school, err)
		}
	}
	return nil
}

// Audiences lists the schools of the Träger with their groups, each read in
// its school's tenant transaction.
func (d *TraegerDocuments) Audiences(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerDocumentSchool, error) {
	if organizationID <= 0 {
		return nil, errTraegerRequired
	}
	schools, err := d.ports.Schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("organization tenancy: list schools of the Träger: %w", err)
	}
	for i := range schools {
		groups, groupErr := d.schoolGroups(ctx, schools[i].ID)
		if groupErr != nil {
			return nil, groupErr
		}
		schools[i].Groups = groups
	}
	return schools, nil
}

func (d *TraegerDocuments) schoolGroups(ctx context.Context, schoolID int64) ([]organizationtenancy.TraegerDocumentGroup, error) {
	var groups []organizationtenancy.TraegerDocumentGroup
	err := d.ports.Runtime.WithinSchool(ctx, schoolID, func(txCtx context.Context) error {
		var listErr error
		groups, listErr = d.ports.Groups.SchoolGroups(txCtx)
		return listErr
	})
	if err != nil {
		return nil, fmt.Errorf("organization tenancy: list groups of school %d: %w", schoolID, err)
	}
	slices.SortFunc(groups, func(a, b organizationtenancy.TraegerDocumentGroup) int {
		if order := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); order != 0 {
			return order
		}
		return int(a.ID - b.ID)
	})
	if groups == nil {
		groups = []organizationtenancy.TraegerDocumentGroup{}
	}
	return groups, nil
}

// Release releases a document for an audience. A school must be one of the
// Träger's undeleted schools, a group one of that school's groups; the
// group's name is kept with the release.
func (d *TraegerDocuments) Release(ctx context.Context, organizationID, accountID, documentID int64, input organizationtenancy.TraegerDocumentReleaseInput) (organizationtenancy.TraegerDocumentRelease, error) {
	if organizationID <= 0 {
		return organizationtenancy.TraegerDocumentRelease{}, errTraegerRequired
	}
	if !domain.ValidTraegerDocumentRelease(string(input.Audience), input.SchoolID, input.GroupID) {
		return organizationtenancy.TraegerDocumentRelease{}, organizationtenancy.ErrTraegerDocumentAudienceInvalid
	}
	groupName, err := d.audienceGroupName(ctx, organizationID, input)
	if err != nil {
		return organizationtenancy.TraegerDocumentRelease{}, err
	}
	var release organizationtenancy.TraegerDocumentRelease
	err = d.ports.Runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var insertErr error
		release, insertErr = d.store.InsertRelease(txCtx, organizationID, accountID, documentID, input, groupName)
		return insertErr
	})
	return release, err
}

// audienceGroupName checks the school and the group of a release and
// returns the group's name, empty without a group.
func (d *TraegerDocuments) audienceGroupName(ctx context.Context, organizationID int64, input organizationtenancy.TraegerDocumentReleaseInput) (string, error) {
	if input.SchoolID == nil {
		return "", nil
	}
	schools, err := d.ports.Schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return "", fmt.Errorf("organization tenancy: list schools of the Träger: %w", err)
	}
	if !slices.ContainsFunc(schools, func(school organizationtenancy.TraegerDocumentSchool) bool { return school.ID == *input.SchoolID }) {
		return "", organizationtenancy.ErrTraegerDocumentAudienceInvalid
	}
	if input.GroupID == nil {
		return "", nil
	}
	groups, err := d.schoolGroups(ctx, *input.SchoolID)
	if err != nil {
		return "", err
	}
	index := slices.IndexFunc(groups, func(group organizationtenancy.TraegerDocumentGroup) bool { return group.ID == *input.GroupID })
	if index < 0 {
		return "", organizationtenancy.ErrTraegerDocumentAudienceInvalid
	}
	return groups[index].Name, nil
}

// DeleteDocument deletes a document of the Träger in one Träger transaction: the
// releases end, the file goes, the deletion is recorded, all or nothing.
func (d *TraegerDocuments) DeleteDocument(ctx context.Context, organizationID, accountID, documentID int64) error {
	if organizationID <= 0 {
		return errTraegerRequired
	}
	return d.ports.Runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		deleted, err := d.store.DeleteDocument(txCtx, organizationID, accountID, documentID)
		if err == nil && !deleted {
			err = organizationtenancy.ErrTraegerDocumentNotFound
		}
		return err
	})
}

// Revoke revokes an active release of a document.
func (d *TraegerDocuments) Revoke(ctx context.Context, organizationID, accountID, documentID, releaseID int64) error {
	if organizationID <= 0 {
		return errTraegerRequired
	}
	return d.ports.Runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		revoked, err := d.store.RevokeRelease(txCtx, organizationID, accountID, documentID, releaseID)
		if err == nil && !revoked {
			err = organizationtenancy.ErrTraegerDocumentReleaseNotFound
		}
		return err
	})
}

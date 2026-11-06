package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// maxBillingSpan bounds VersionsInForce: a billing run asks for a month, a
// due date for a day.
const maxBillingSpan = 400

// VersionDocument answers the PDF of one version of a contract and records
// the read in the trail and in the access log of the contract's school.
func (c *Contracts) VersionDocument(ctx context.Context, reviewer carecontract.Reviewer, contractID int64, number int) (carecontract.File, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.File{}, err
	}
	var file carecontract.File
	var schoolID int64
	err := c.runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		var err error
		file, schoolID, err = c.versionDocument(txCtx, reviewer, contractID, number)
		return err
	})
	if err != nil {
		return carecontract.File{}, err
	}
	return file, c.logSchools(ctx, reviewer, "care_contract_version_document", map[int64][]int64{schoolID: {contractID}})
}

// versionDocument is the stored PDF of a sent version, else a rendering of
// the version with the current template that stores nothing.
func (c *Contracts) versionDocument(ctx context.Context, reviewer carecontract.Reviewer, contractID int64, number int) (carecontract.File, int64, error) {
	record, err := c.store.Contract(ctx, reviewer.OrganizationID, contractID, false)
	if err != nil {
		return carecontract.File{}, 0, err
	}
	versions, err := c.store.Versions(ctx, reviewer.OrganizationID, contractID)
	if err != nil {
		return carecontract.File{}, 0, err
	}
	var version *carecontract.Version
	for i := range versions {
		if versions[i].Number == number {
			version = &versions[i]
		}
	}
	if version == nil {
		return carecontract.File{}, 0, carecontract.ErrVersionNotFound
	}
	file, stored, err := c.store.VersionDocument(ctx, reviewer.OrganizationID, version.ID)
	if err != nil {
		return carecontract.File{}, 0, err
	}
	if !stored {
		record.Version = *version
		if file, _, err = c.render(ctx, reviewer, record); err != nil {
			return carecontract.File{}, 0, err
		}
	}
	view := "version_preview"
	if stored {
		view = "version_document"
	}
	if err := c.record(ctx, reviewer, record, ActionRead, map[string]any{"view": view, "version_number": number}); err != nil {
		return carecontract.File{}, 0, err
	}
	return file, record.Summary.SchoolID, nil
}

// sentVersionRecord points a send again at the PDF the parents have (#4681):
// once a later version holds, the current version has no stored PDF, so the
// record carries the version and document of the newest stored PDF instead.
// A contract whose current version has its PDF, or that has none yet, stays
// as it is.
func (c *Contracts) sentVersionRecord(ctx context.Context, organizationID int64, record ContractRecord) (ContractRecord, error) {
	if record.DocumentID != nil || record.SentDocument == nil {
		return record, nil
	}
	versions, err := c.store.Versions(ctx, organizationID, record.Summary.ID)
	if err != nil {
		return ContractRecord{}, err
	}
	return withSentVersion(record, versions), nil
}

// withSentVersion replaces the record's version by the one its newest stored
// PDF shows.
func withSentVersion(record ContractRecord, versions []carecontract.Version) ContractRecord {
	for _, version := range versions {
		if version.ID == record.SentDocument.VersionID {
			documentID := record.SentDocument.ID
			record.Version, record.DocumentID = version, &documentID
		}
	}
	return record
}

// Versions serves carecontract.ContractVersions for the parents' billing
// (#4684) in the caller's Träger transaction.
type Versions struct {
	store Store
}

var _ carecontract.ContractVersions = (*Versions)(nil)

// NewVersions builds the read of the versions in force.
func NewVersions(store Store) *Versions {
	if store == nil {
		panic("care contract: the store is required")
	}
	return &Versions{store: store}
}

// VersionsInForce answers each version in force between from and to with
// the days it holds within that span.
func (v *Versions) VersionsInForce(ctx context.Context, organizationID int64, from, to calendar.Date, contractIDs ...int64) ([]carecontract.VersionPeriod, error) {
	if organizationID <= 0 || from.IsZero() || to.IsZero() || to.Before(from) || from.DaysUntil(to) > maxBillingSpan {
		return nil, &carecontract.ValidationError{Fields: map[string]string{"period": "invalid"}}
	}
	return v.periods(ctx, organizationID, from, to, contractIDs)
}

// periods reads the versions in force and clips each to the span.
func (v *Versions) periods(ctx context.Context, organizationID int64, from, to calendar.Date, contractIDs []int64) ([]carecontract.VersionPeriod, error) {
	periods, err := v.store.VersionsInForce(ctx, organizationID, from, to, contractIDs)
	if err != nil {
		return nil, fmt.Errorf("care contract: versions in force: %w", err)
	}
	for i := range periods {
		periods[i].From, periods[i].Until = clipPeriod(periods[i].Version, from, to)
	}
	return periods, nil
}

// clipPeriod is the part of the version's validity within from and to.
func clipPeriod(version carecontract.Version, from, to calendar.Date) (calendar.Date, calendar.Date) {
	start, end := version.EffectiveFrom, to
	if start.Before(from) {
		start = from
	}
	if version.ValidUntil != nil && version.ValidUntil.Before(end) {
		end = *version.ValidUntil
	}
	return start, end
}

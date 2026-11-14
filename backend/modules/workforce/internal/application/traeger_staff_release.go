package application

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/ports"
)

// TraegerStaffReleases is the release „Personal beim Träger“ the moto team
// grants and withdraws in the Operator portal (ADR 0089 section 2). Every
// call joins the caller's administrative transaction, which writes the
// operator audit entry of a change before the change.
type TraegerStaffReleases struct {
	store ports.TraegerStaffStore
}

// NewTraegerStaffReleases composes the release over the store.
func NewTraegerStaffReleases(store ports.TraegerStaffStore) *TraegerStaffReleases {
	if store == nil {
		panic("workforce traeger staff releases: store is required")
	}
	return &TraegerStaffReleases{store: store}
}

var _ workforce.TraegerStaffReleases = (*TraegerStaffReleases)(nil)

// ReleaseState returns the release of the Träger and how many records it
// keeps.
func (r *TraegerStaffReleases) ReleaseState(ctx context.Context, organizationID int64) (workforce.TraegerStaffReleaseState, error) {
	release, err := r.store.ReleaseOfTraeger(ctx, organizationID)
	if err != nil {
		return workforce.TraegerStaffReleaseState{}, err
	}
	records, err := r.store.CountStaffRecords(ctx, organizationID)
	if err != nil {
		return workforce.TraegerStaffReleaseState{}, err
	}
	state := workforce.TraegerStaffReleaseState{Records: records}
	if release != nil {
		state.Release = publicRelease(*release)
	}
	return state, nil
}

// GrantRelease records the release with the statement the Träger confirmed.
func (r *TraegerStaffReleases) GrantRelease(ctx context.Context, organizationID, operatorID int64, representation string) (workforce.TraegerStaffRelease, error) {
	if representation != workforce.TraegerStaffRepresentationAgreement && representation != workforce.TraegerStaffRepresentationNone {
		return workforce.TraegerStaffRelease{}, workforce.ErrTraegerStaffReleaseInvalid
	}
	if organizationID <= 0 || operatorID <= 0 {
		return workforce.TraegerStaffRelease{}, errors.New("workforce traeger staff releases: Träger and operator are required")
	}
	release, err := r.store.InsertRelease(ctx, domain.TraegerStaffRelease{
		OrganizationID: organizationID, Representation: representation, ReleasedByOperatorID: operatorID,
	})
	if err != nil {
		return workforce.TraegerStaffRelease{}, publicTraegerStaffError(err)
	}
	return *publicRelease(release), nil
}

// WithdrawRelease removes the release; while the Träger keeps records it
// refuses with workforce.ErrTraegerStaffReleaseInUse. The count answers the
// usual case; the records' foreign key refuses a record linked meanwhile.
func (r *TraegerStaffReleases) WithdrawRelease(ctx context.Context, organizationID int64) error {
	records, err := r.store.CountStaffRecords(ctx, organizationID)
	if err != nil {
		return err
	}
	if records > 0 {
		return workforce.ErrTraegerStaffReleaseInUse
	}
	removed, err := r.store.DeleteRelease(ctx, organizationID)
	if err != nil {
		return publicTraegerStaffError(err)
	}
	if !removed {
		return workforce.ErrTraegerStaffReleaseNotFound
	}
	return nil
}

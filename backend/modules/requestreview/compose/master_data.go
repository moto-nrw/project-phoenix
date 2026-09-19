package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carecompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/uptrace/bun"
)

type ReviewScope = carecompose.ReviewScope
type ReviewDate = careplan.Date
type CareObservation = carecompose.Observation

// NewMasterDataReviews binds the native owner workflow to People Directory.
func NewMasterDataReviews(db *bun.DB, people MasterDataPeople, scope carecompose.ReviewScopeResolver, today func() careplan.Date, observe func(CareObservation)) (careplan.MasterDataReviewQuery, error) {
	requests, err := carecompose.NewStudentDataRequestQueries(db, observe)
	if err != nil {
		return nil, err
	}
	var directory carecompose.MasterDataDirectory
	if people != nil {
		directory = masterDataDirectory{reviewDirectory: reviewDirectory{people: people}, fields: people}
	}
	return carecompose.NewMasterDataReviews(carecompose.MasterDataReviewDependencies{
		Requests: requests, People: directory, Scope: scope, Today: today,
	})
}

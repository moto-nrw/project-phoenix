package compose

import "github.com/moto-nrw/project-phoenix/modules/careplan"

// NewMasterDataReviewDirectory binds the native review directory port.
func NewMasterDataReviewDirectory(people MasterDataPeople) careplan.MasterDataDirectory {
	if people == nil {
		return nil
	}
	return masterDataDirectory{reviewDirectory: reviewDirectory{people: people}, fields: people}
}

//go:build testhooks

package application

import "errors"

// NewCareOfferingCatalog builds the catalog over its ports and resyncers.
func NewCareOfferingCatalog(deps CareOfferingCatalogDependencies) (*CareOfferingCatalog, error) {
	if deps.SourcedTemplates == nil || deps.Pickup == nil {
		return nil, errors.New("care offering catalog: the sourced-template and pickup resyncers are required")
	}
	return newCareOfferingCatalog(deps)
}

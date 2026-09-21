package enrollment

import (
	"encoding/json"
	"fmt"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Care offerings carry their translations (#3377) as stored JSON because the
// catalog rows belong to Care Plan, which must not depend on enrollment
// types. These helpers are the only place that document is interpreted.

// CareOfferingTranslations decodes the stored translation document.
func CareOfferingTranslations(offering *enrollmentModels.CareOffering) (enrollmentOwner.Translations, error) {
	if offering == nil || len(offering.Translations) == 0 {
		return nil, nil
	}
	var translations enrollmentOwner.Translations
	if err := json.Unmarshal(offering.Translations, &translations); err != nil {
		return nil, fmt.Errorf("decode care offering translations: %w", err)
	}
	return translations, nil
}

// CareOfferingPublicTranslations returns the translations parents may read:
// only those still matching the current German name and description.
func CareOfferingPublicTranslations(offering *enrollmentModels.CareOffering) (enrollmentOwner.Translations, error) {
	translations, err := CareOfferingTranslations(offering)
	if err != nil || translations == nil {
		return nil, err
	}
	sources := map[string]string{
		enrollmentOwner.TranslationAttrName:           offering.Name,
		enrollmentOwner.TranslationAttrSelectionGroup: offering.SelectionGroup,
	}
	if offering.Description != nil {
		sources[enrollmentOwner.TranslationAttrDescription] = *offering.Description
	}
	return translations.Fresh(sources), nil
}

// normalizeCareOfferingTranslations validates the submitted document and
// stores its canonical form, or nothing when no translation remains.
func normalizeCareOfferingTranslations(offering *enrollmentModels.CareOffering) error {
	translations, err := CareOfferingTranslations(offering)
	if err != nil {
		return careOfferingInvalidf("translations must be a valid translation document")
	}
	translations, err = translations.Normalize(
		enrollmentOwner.TranslationAttrName,
		enrollmentOwner.TranslationAttrDescription,
		enrollmentOwner.TranslationAttrSelectionGroup,
	)
	if err != nil {
		return wrapCareOfferingInvalid(err, "validate care offering translations")
	}
	if translations == nil {
		offering.Translations = nil
		return nil
	}
	offering.Translations, err = json.Marshal(translations)
	if err != nil {
		return fmt.Errorf("encode care offering translations: %w", err)
	}
	return nil
}

package application

import (
	"fmt"

	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// holidayCareWishAsked reports whether the form asks the holiday care wish
// of every child (#4703).
func holidayCareWishAsked(schema *enrollmentNative.FormSchema) bool {
	return schema != nil && schema.CoreRequirements.Required(enrollmentNative.CoreRequirementHolidayCareWish)
}

// validateHolidayCareWish requires an answer for every child when the form
// asks it.
func validateHolidayCareWish(schema *enrollmentNative.FormSchema, children []SubmitChild) error {
	if !holidayCareWishAsked(schema) {
		return nil
	}
	for i := range children {
		if children[i].HolidayCareWish == nil {
			return enrollmentNative.InvalidInput(enrollmentNative.CodeHolidayCareWishRequired, fmt.Sprintf("children.%d.holiday_care_wish", i),
				fmt.Errorf("%w: child %d holiday care wish is required", enrollmentNative.ErrInvalidSubmission, i))
		}
	}
	return nil
}

// dropUnaskedHolidayCareWish removes an answer the form did not ask, like
// the answers of hidden fields: a scripted client cannot store a wish the
// school never asked for.
func dropUnaskedHolidayCareWish(schema *enrollmentNative.FormSchema, children []SubmitChild) {
	if holidayCareWishAsked(schema) {
		return
	}
	for i := range children {
		children[i].HolidayCareWish = nil
	}
}

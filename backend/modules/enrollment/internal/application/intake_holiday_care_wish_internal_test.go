package application

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// A form that asks the holiday care wish (#4703) requires an answer for
// every child, and a form that does not ask it stores none, even when a
// scripted client sends one. Each case goes red without its check.
func TestHolidayCareWishIsRequiredWhenAskedAndDroppedOtherwise(t *testing.T) {
	t.Parallel()
	yes := true
	asks := &enrollmentNative.FormSchema{CoreRequirements: enrollmentNative.CoreRequirements{
		enrollmentNative.CoreRequirementHolidayCareWish: true,
	}}

	err := validateRequiredCustomFields(asks, SubmitRequest{Children: []SubmitChild{{HolidayCareWish: &yes}, {}}}, nil)
	var rejection *enrollmentNative.InvalidInputError
	require.True(t, errors.As(err, &rejection), "the second child has no answer: %v", err)
	assert.Equal(t, enrollmentNative.CodeHolidayCareWishRequired, rejection.Code)
	assert.Equal(t, "children.1.holiday_care_wish", rejection.Field)
	assert.NoError(t, validateRequiredCustomFields(asks, SubmitRequest{Children: []SubmitChild{{HolidayCareWish: &yes}}}, nil))

	silent := &enrollmentNative.FormSchema{}
	assert.NoError(t, validateRequiredCustomFields(silent, SubmitRequest{Children: []SubmitChild{{}}}, nil),
		"a form that does not ask requires nothing")
	for _, schema := range []*enrollmentNative.FormSchema{silent, nil} {
		children := []SubmitChild{{HolidayCareWish: &yes}}
		dropUnaskedHolidayCareWish(schema, children)
		assert.Nil(t, children[0].HolidayCareWish, "an unasked answer is not stored")
	}
	children := []SubmitChild{{HolidayCareWish: &yes}}
	dropUnaskedHolidayCareWish(asks, children)
	require.NotNil(t, children[0].HolidayCareWish)
	assert.True(t, *children[0].HolidayCareWish)
}

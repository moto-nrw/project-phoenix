package education

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestClassArrivalExceptionLabel(t *testing.T) {
	t.Parallel()

	reason := "  Unterricht fällt aus "
	assert.Equal(t, "Klasse 4a: Unterricht fällt aus", (&ClassArrivalException{SchoolClass: "4a", Reason: &reason}).Label())
	assert.Equal(t, "Klasse 2a: Unterricht fällt aus", (&ClassArrivalException{SchoolClass: "Klasse 2a", Reason: &reason}).Label(),
		"a class already named 'Klasse 2a' must not read 'Klasse Klasse 2a'")
	assert.Equal(t, "Klasse 3b: andere Ankunftszeit", (&ClassArrivalException{SchoolClass: " 3b "}).Label())
	empty := "   "
	assert.Equal(t, "Klasse 3b: andere Ankunftszeit", (&ClassArrivalException{SchoolClass: "3b", Reason: &empty}).Label())
}

func TestClassArrivalExceptionValidate(t *testing.T) {
	t.Parallel()

	noon := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
	valid := &ClassArrivalException{SchoolClass: "4a", Date: timezone.NewDate(2027, 3, 1), ArrivalTime: noon}
	assert.NoError(t, valid.Validate())

	assert.Error(t, (&ClassArrivalException{SchoolClass: " ", Date: timezone.NewDate(2027, 3, 1), ArrivalTime: noon}).Validate())
	assert.Error(t, (&ClassArrivalException{SchoolClass: "4a", ArrivalTime: noon}).Validate())
	assert.Error(t, (&ClassArrivalException{SchoolClass: "4a", Date: timezone.NewDate(2027, 3, 1)}).Validate())
	long := strings.Repeat("x", 256)
	assert.Error(t, (&ClassArrivalException{SchoolClass: "4a", Date: timezone.NewDate(2027, 3, 1), ArrivalTime: noon, Reason: &long}).Validate())
}

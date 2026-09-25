package application

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
)

// TestMasterDataPersonValue pins how a live person field reads against a
// request's stored baseline (#1803).
func TestMasterDataPersonValue(t *testing.T) {
	t.Parallel()

	person := &ports.MasterDataPerson{FirstName: "Max", LastName: "Mustermann", Birthday: "2018-05-04"}
	for field, want := range map[string]string{
		"first_name": `"Max"`,
		"last_name":  `"Mustermann"`,
		"birthday":   `"2018-05-04"`,
	} {
		got, err := masterDataPersonValue(person, field)
		if err != nil || string(got) != want {
			t.Errorf("%s = (%s, %v), want (%s, nil)", field, got, err, want)
		}
	}

	// A missing birthday must read as JSON null, not the zero date.
	got, err := masterDataPersonValue(&ports.MasterDataPerson{}, "birthday")
	if err != nil || string(got) != "null" {
		t.Errorf("missing birthday = (%s, %v), want (null, nil)", got, err)
	}

	// An unsupported field is a hard "not applicable", not a silent empty value.
	if _, err := masterDataPersonValue(person, "shoe_size"); !errors.Is(err, masterdatarequests.ErrReviewInvalidTarget) {
		t.Errorf("unknown field err = %v, want ErrReviewInvalidTarget", err)
	}
}

// TestDecodeRequestedDeparture refuses an undecodable plan and the
// "geht mit" mode a request may not set.
func TestDecodeRequestedDeparture(t *testing.T) {
	t.Parallel()

	for name, raw := range map[string]string{
		"malformed":   `not-json`,
		"accompanied": `{"mon":["accompanied"]}`,
	} {
		if _, err := decodeRequestedDeparture([]byte(raw)); !errors.Is(err, masterdatarequests.ErrReviewInvalidValue) {
			t.Errorf("%s err = %v, want ErrReviewInvalidValue", name, err)
		}
	}
	modes, err := decodeRequestedDeparture([]byte(`{"mon":["bus"]}`))
	if err != nil || len(modes["mon"]) != 1 {
		t.Errorf("valid plan = (%v, %v)", modes, err)
	}
}

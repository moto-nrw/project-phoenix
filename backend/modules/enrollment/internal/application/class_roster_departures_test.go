package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// TestClassRosterRowNamesStructuredCompanions pins that the class roster prints
// WHO an accompanied child walks home with. The structured links satisfy the
// "mit wem" requirement per weekday, so such a child legitimately has no
// free-text note — a roster built from the note alone would hand staff a sheet
// that says "Mit anderem Kind" and no name (#1694).
func TestClassRosterRowNamesStructuredCompanions(t *testing.T) {
	t.Parallel()

	student := &RosterStudent{
		ID:          101,
		PersonID:    201,
		SchoolClass: "1a",
		AllowedDepartureModes: DepartureModes{
			"mon": {"accompanied"},
		},
	}
	person := &RosterPerson{FirstName: "Lina", LastName: "Läuft"}
	companions := []CompanionLink{
		{CompanionStudentID: 7, FirstName: "Mia", LastName: "Schulz", Weekdays: []string{"mon"}},
	}

	row, err := classRosterRow(student, person, "", nil, nil, nil, nil, companions, true)

	require.NoError(t, err)
	assert.Equal(t, "mit anderem Kind (mit: Mia Schulz)", row.DepartureByDay["mon"])
	assert.Equal(t, "geht alleine", row.DepartureByDay["tue"])
}

// TestClassRosterRowCombinesCompanionsAndNote pins that a child using both
// sources keeps both: the link answers Monday, the note answers the day no link
// covers: Monday prints the link, Tuesday falls back to the note — the note
// never repeats on a day a link already names.
func TestClassRosterRowCombinesCompanionsAndNote(t *testing.T) {
	t.Parallel()

	note := "Dienstags mit Nachbarskind Tom"
	student := &RosterStudent{
		ID:          101,
		PersonID:    201,
		SchoolClass: "1a",
		AllowedDepartureModes: DepartureModes{
			"mon": {"accompanied"},
			"tue": {"accompanied"},
		},
		DepartureCompanionNote: &note,
	}
	person := &RosterPerson{FirstName: "Lina", LastName: "Läuft"}
	companions := []CompanionLink{
		{CompanionStudentID: 7, FirstName: "Mia", LastName: "Schulz", Weekdays: []string{"mon"}},
	}

	row, err := classRosterRow(student, person, "", nil, nil, nil, nil, companions, true)

	require.NoError(t, err)
	assert.Equal(t, "mit anderem Kind (mit: Mia Schulz)", row.DepartureByDay["mon"])
	assert.Equal(t, "mit anderem Kind (mit: Dienstags mit Nachbarskind Tom)", row.DepartureByDay["tue"])
}

// TestClassRosterRowDropsCompanionDaysOutsideThePhasePlan pins that the roster
// never prints a "läuft mit" day the plan on the same line forbids. The plan
// comes from the approved enrollment phase while the links belong to the live
// child and follow every later Stammdaten edit, so the two sources can disagree
// — and a Tuesday cell that says "fährt Bus" and "dienstags mit Mia" at once
// is worse than one that says nothing about Tuesday (#1694).
func TestClassRosterRowDropsCompanionDaysOutsideThePhasePlan(t *testing.T) {
	t.Parallel()

	schemaID := int64(88)
	req := &enrollmentModels.Request{
		ID:          10,
		SchemaID:    &schemaID,
		SubmittedAt: time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC),
	}
	child := &reportChild{
		ID:        20,
		RequestID: 10,
		FirstName: "Lina",
		LastName:  "Muster",
		Status:    enrollmentModels.ChildStatusApproved,
		CustomData: map[string]any{
			"departure": map[string]any{
				"mon": []any{"accompanied"},
				"tue": []any{"bus"},
			},
		},
	}
	student := &RosterStudent{
		ID:          100,
		PersonID:    200,
		SchoolClass: "1a",
	}
	person := &RosterPerson{FirstName: "Lina", LastName: "Muster"}
	approved := &classRosterApprovedEnrollment{request: req, child: child}
	schemas := map[int64]*enrollment.FormSchema{
		schemaID: {
			Fields: []enrollment.FormField{
				{Key: "departure", Target: enrollment.TargetStudentAllowedDepartureModes, Type: enrollment.FormFieldWeekdayMultiMode, AppliesToCh: true},
			},
		},
	}
	companions := []CompanionLink{
		{CompanionStudentID: 7, FirstName: "Mia", LastName: "Schulz", Weekdays: []string{"mon", "tue"}},
	}

	row, err := classRosterRow(student, person, "", approved, nil, schemas, nil, companions, true)

	require.NoError(t, err)
	assert.Equal(t, "mit anderem Kind (mit: Mia Schulz)", row.DepartureByDay["mon"])
	assert.Equal(t, "fährt Bus", row.DepartureByDay["tue"])
}

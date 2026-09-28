package users

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/userscontract"
)

// The model-free error vocabulary of this package lives in People Directory's
// userscontract package (#3750). These tests pin that the retained names stay
// aliases of the moved instances, so errors.Is and errors.As keep matching for
// every existing caller and no error string changes.
func TestErrorSentinelsAreAliasesOfUsersContract(t *testing.T) {
	t.Parallel()

	pairs := []struct {
		name    string
		legacy  error
		moved   error
		message string
	}{
		{"ErrPersonIdentifierRequired", ErrPersonIdentifierRequired, userscontract.ErrPersonIdentifierRequired, "either tag ID or account ID is required"},
		{"ErrAccountNotFound", ErrAccountNotFound, userscontract.ErrAccountNotFound, "account not found"},
		{"ErrRFIDCardNotFound", ErrRFIDCardNotFound, userscontract.ErrRFIDCardNotFound, "RFID card not found"},
		{"ErrAccountAlreadyLinked", ErrAccountAlreadyLinked, userscontract.ErrAccountAlreadyLinked, "account is already linked to another person"},
		{"ErrRFIDCardAlreadyLinked", ErrRFIDCardAlreadyLinked, userscontract.ErrRFIDCardAlreadyLinked, "RFID card is already linked to another person"},
		{"ErrTeacherNotFound", ErrTeacherNotFound, userscontract.ErrTeacherNotFound, "teacher not found"},
		{"ErrStaffNotFound", ErrStaffNotFound, userscontract.ErrStaffNotFound, "staff not found"},
		{"ErrStaffAdoptionNotPermitted", ErrStaffAdoptionNotPermitted, userscontract.ErrStaffAdoptionNotPermitted, "Für das Ändern eines vorhandenen Mitarbeiter-Datensatzes fehlt die Berechtigung"},
		{"ErrStaffLehrkraftCaregiverProfile", ErrStaffLehrkraftCaregiverProfile, userscontract.ErrStaffLehrkraftCaregiverProfile, "Ein Lehrkraft-Konto kann kein Betreuungsprofil erhalten"},
		{"ErrStudentGraduated", ErrStudentGraduated, userscontract.ErrStudentGraduated, "student has graduated"},
		{"ErrCompanionNotFound", ErrCompanionNotFound, userscontract.ErrCompanionNotFound, "Ein ausgewähltes Kind wurde nicht gefunden."},
		{"ErrDuplicateCompanion", ErrDuplicateCompanion, userscontract.ErrDuplicateCompanion, "Ein Kind kann nur einmal in der Laufgemeinschaft stehen."},
		{"ErrCompanionWeekdayRequired", ErrCompanionWeekdayRequired, userscontract.ErrCompanionWeekdayRequired, "Bitte mindestens einen Wochentag auswählen."},
		{"ErrCompanionDayNotAllowed", ErrCompanionDayNotAllowed, userscontract.ErrCompanionDayNotAllowed, "An diesem Tag ist \"Anderes Kind\" als Heimweg nicht erlaubt."},
		{"ErrTooManyCompanions", ErrTooManyCompanions, userscontract.ErrTooManyCompanions, "Es können höchstens 10 Kinder verknüpft werden."},
		{"ErrCompanionAtLimit", ErrCompanionAtLimit, userscontract.ErrCompanionAtLimit, "Ein ausgewähltes Kind ist bereits mit 10 Kindern verknüpft."},
		{"ErrCompanionsChanged", ErrCompanionsChanged, userscontract.ErrCompanionsChanged, "Die Laufgemeinschaft dieses Kindes wurde zwischenzeitlich geändert. Bitte neu laden und noch einmal speichern."},
		{"ErrCompanionExtensionNotAuthorized", ErrCompanionExtensionNotAuthorized, userscontract.ErrCompanionExtensionNotAuthorized, "companion departure-plan extension was not authorized"},
		{"ErrStaffInUse", ErrStaffInUse, userscontract.ErrStaffInUse, "Personal kann nicht gelöscht werden: Mitarbeiter/in hat aktive Aufsichten oder Anwesenheitseinträge"},
	}

	for _, tt := range pairs {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Same(t, tt.moved, tt.legacy, "the retained name must be the moved instance")
			assert.Equal(t, tt.message, tt.legacy.Error())
		})
	}
}

func TestErrorTypesAreAliasesOfUsersContract(t *testing.T) {
	t.Parallel()

	var usersErr error = &UsersError{Op: "GetPerson", Err: ErrPersonNotFound}
	var moved *userscontract.UsersError
	assert.True(t, errors.As(usersErr, &moved), "errors.As must match the moved UsersError")
	assert.Equal(t, "users.GetPerson: person not found", usersErr.Error())
	assert.ErrorIs(t, usersErr, ErrPersonNotFound)

	var validation error = &ValidationError{Err: ErrStudentGraduated}
	var movedValidation *userscontract.ValidationError
	assert.True(t, errors.As(validation, &movedValidation))
	assert.ErrorIs(t, validation, userscontract.ErrStudentGraduated)
	assert.Equal(t, "student has graduated", validation.Error())
}

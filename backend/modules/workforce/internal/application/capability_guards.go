package application

import (
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

func invalidStaffRecord(reason string) error {
	return &workforce.InvalidStaffRecordError{Reason: reason}
}
func invalidShift(reason string) error   { return &workforce.InvalidStaffShiftError{Reason: reason} }
func invalidAbsence(reason string) error { return &workforce.InvalidStaffAbsenceError{Reason: reason} }
func invalidWorkSession(reason string) error {
	return &workforce.InvalidWorkSessionError{Reason: reason}
}
func invalidSubstitution(reason string) error {
	return &workforce.InvalidGroupSubstitutionError{Reason: reason}
}
func invalid(reason string) error       { return &workforce.InvalidWorkTimeError{Reason: reason} }
func invalidSeries(reason string) error { return &workforce.InvalidShiftSeriesError{Reason: reason} }

func validateWorkSessionDateColumn(column string) error {
	if column != workforce.WorkSessionDateColumn {
		return invalidWorkSession("unsupported date column " + column)
	}
	return nil
}

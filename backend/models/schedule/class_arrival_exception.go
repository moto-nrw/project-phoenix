package schedule

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// classArrivalExceptionReasonMaxLength caps the free-text reason, mirroring
// schedule.StudentArrivalException.Reason.
const classArrivalExceptionReasonMaxLength = 255

// ClassArrivalException is one day on which a whole school class arrives at a
// different time than its weekly Unterrichtsschluss says (#2962): Unterricht
// fällt aus, the class is released early, a Wandertag ends late. It replaces
// the class time of that date inside the arrival projection (ADR 0005), so
// every reader of the effective arrival time sees it without its own lookup.
// A per-child day exception (schedule.student_arrival_exceptions) still wins.
//
// Like ClassArrivalTime the class is the free-text users.students.school_class
// value, matched via LOWER(BTRIM(...)); there is no class entity.
type ClassArrivalException struct {
	base.Model `bun:"schema:education,table:class_arrival_exceptions"`
	base.TenantModel

	SchoolClass string        `bun:"school_class,notnull" json:"school_class"`
	Date        timezone.Date `bun:"date,notnull,type:date" json:"date"`
	// ArrivalTime is a wall-clock TIME column; normalize via
	// timezone.NormalizeWallClock before persisting.
	ArrivalTime time.Time `bun:"arrival_time,notnull" json:"arrival_time"`
	Reason      *string   `bun:"reason" json:"reason,omitempty"`
	CreatedBy   *int64    `bun:"created_by,nullzero" json:"created_by,omitempty"`
	// Origin is the portal that entered the row (#2970): the OGS portal or
	// "moto schule". Readers show "eingetragen von der Schule" for the
	// latter; created_by alone cannot tell the two apart. Empty binds the
	// column default (ogs).
	Origin string `bun:"origin,notnull,nullzero,default:'ogs'" json:"origin"`
}

// Origin values of ClassArrivalException.Origin.
const (
	// ClassArrivalExceptionOriginOGS marks a row entered in the OGS portal.
	ClassArrivalExceptionOriginOGS = "ogs"
	// ClassArrivalExceptionOriginSchool marks a row a Lehrkraft entered
	// through "moto schule" (#2970).
	ClassArrivalExceptionOriginSchool = "school"
)

// Validate rejects an empty class, a missing date, a missing time and an
// overlong reason without mutating the model.
func (e *ClassArrivalException) Validate() error {
	if strings.TrimSpace(e.SchoolClass) == "" {
		return errors.New("school class is required")
	}
	if e.Date.IsZero() {
		return errors.New("date is required")
	}
	if e.ArrivalTime.IsZero() {
		return errors.New("arrival time is required")
	}
	if e.Reason != nil && utf8.RuneCountInString(*e.Reason) > classArrivalExceptionReasonMaxLength {
		return errors.New("reason cannot exceed 255 characters")
	}
	if e.Origin != "" && e.Origin != ClassArrivalExceptionOriginOGS && e.Origin != ClassArrivalExceptionOriginSchool {
		return errors.New("origin must be ogs or school")
	}
	return nil
}

// Label is the one line every reader shows next to the changed time, for
// example "Klasse 4a: Unterricht fällt aus". Without a reason it still names
// the class, so a Betreuungskraft can tell a class-wide change from a
// per-child one.
func (e *ClassArrivalException) Label() string {
	// Schools name their classes either "3b" or "Klasse 3b"; without the
	// strip the label reads "Klasse Klasse 3b".
	class := strings.TrimSpace(e.SchoolClass)
	if len(class) > 7 && strings.EqualFold(class[:7], "klasse ") {
		class = strings.TrimSpace(class[7:])
	}
	if e.Reason != nil && strings.TrimSpace(*e.Reason) != "" {
		return "Klasse " + class + ": " + strings.TrimSpace(*e.Reason)
	}
	return "Klasse " + class + ": andere Ankunftszeit"
}

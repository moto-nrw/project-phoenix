package ports

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ErrCatalogRowNotFound marks a phase, activity group or calendar period the
// catalog asked for that does not exist in the tenant. The composition wraps
// the owner's own not-found error with it and keeps its text.
var ErrCatalogRowNotFound = errors.New("care offering catalog: row not found")

// CatalogRecords is Care Plan's own care-offering persistence.
type CatalogRecords interface {
	FindCareOffering(context.Context, int64) (careplan.CareOffering, error)
	ListCareOfferings(context.Context, careplan.CareOfferingFilter) ([]careplan.CareOffering, error)
	CreateCareOffering(context.Context, careplan.CreateCareOffering) (careplan.CareOffering, error)
	UpdateCareOffering(context.Context, careplan.UpdateCareOffering) (careplan.CareOffering, error)
	DeleteCareOffering(context.Context, int64) error
	ReplaceAutoAddTriggers(context.Context, int64, []int64) error
}

// CatalogPhases reads the Enrollment phase an offering belongs to.
type CatalogPhases interface {
	Phase(ctx context.Context, id int64) (careplan.OfferingPhase, error)
}

// CatalogTimetable reads the Timetable rows a linked offering materializes
// from.
type CatalogTimetable interface {
	FindGroup(ctx context.Context, id int64) (careplan.LinkedGroup, error)
	// TemplateSeries returns the live segments of groupID's split series in
	// id order.
	TemplateSeries(ctx context.Context, groupID int64) ([]careplan.LinkedGroup, error)
	GroupSchedules(ctx context.Context, groupIDs []int64) ([]careplan.LinkedSchedule, error)
	Timeframes(ctx context.Context) ([]careplan.LinkedTimeframe, error)
	ExceptionsBetween(ctx context.Context, from, until calendar.Date) ([]careplan.LinkedException, error)
	GroupExceptions(ctx context.Context, groupID int64) ([]careplan.LinkedException, error)
}

// CatalogCalendar reads School Calendar planning periods and asks the one
// A/B-week engine whether a week pattern occurs on a date.
type CatalogCalendar interface {
	FindPeriod(ctx context.Context, id int64) (careplan.LinkedPeriod, error)
	WeekPatternApplies(weekPattern int, date calendar.Date, period careplan.LinkedPeriod) bool
}

// OfferingGradeCount is how many booked children of one grade level (nil =
// unknown) an offering holds.
type OfferingGradeCount struct {
	CareOfferingID int64
	GradeLevel     *int16
	Count          int
}

// CatalogBookings reads the booking aggregates of Enrollment's selections.
type CatalogBookings interface {
	OfferingGradeCounts(ctx context.Context, offeringIDs []int64, from, until calendar.Date) ([]OfferingGradeCount, error)
	OfferingCapacityPeaks(ctx context.Context, offeringIDs []int64, from, until calendar.Date) (map[int64]int, error)
	// MaterializableOfferingCount counts the non-terminal selections of an
	// offering a later decision still materializes.
	MaterializableOfferingCount(ctx context.Context, offeringID int64, today calendar.Date) (int, error)
}

// CatalogSettings resolves the tenant settings the catalog validates against.
type CatalogSettings interface {
	GradeLevelMax(ctx context.Context) (int, error)
}

// CatalogTranslations interprets the translation document Enrollment owns
// (#3377). ok is false when raw is not a translation document; a nil result
// means no translation remains.
type CatalogTranslations interface {
	NormalizeTranslations(raw json.RawMessage) (normalized json.RawMessage, ok bool, err error)
}

// OfferingSourceRules is the Timetable owner's offering-source contract: the
// cap on sources per template and its refusal, which templates classify.
type OfferingSourceRules interface {
	MaxSourcesPerTemplate() int
	Reject(reason string) error
	IsRejection(err error) bool
}

// SourcedTemplateResyncer keeps the rosters of the templates sourcing an
// offering in step with its edits (#2137/#2147) and retires them before the
// offering is deleted.
type SourcedTemplateResyncer interface {
	ResyncTemplatesSourcedFromOffering(ctx context.Context, offeringID int64, effectiveFrom calendar.Date) error
	DetachTemplatesSourcedFromOffering(ctx context.Context, offeringID int64, effectiveFrom calendar.Date) error
}

// PickupResyncer refreshes the materialized consumers of the date-aware
// pickup projection after an offering edit.
type PickupResyncer interface {
	ReconcileOfferingPickupForOffering(ctx context.Context, offeringID int64) error
}

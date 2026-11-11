// Package traegercatalog is the workflow that copies a Träger phase and its
// Träger care offerings into the Träger's schools (ADR 0195). It owns no
// data: Enrollment and Care Plan keep the Träger objects and the copies,
// Parent Billing the tariffs, Care Contract the contract templates. The
// workflow holds the school list, the preview fingerprint, the lock order
// and the order of the transactions.
package traegercatalog

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ErrPreviewStale rejects a rollout whose preview no longer matches the
// draft or the schools of the Träger phase.
var ErrPreviewStale = errors.New("the preview no longer matches the träger phase")

// Actor is the Träger office of the session.
type Actor struct {
	OrganizationID int64
	AccountID      int64
}

// What a rollout does at one school.
const (
	ActionCreate   = "create"
	ActionUpdate   = "update"
	ActionWithdraw = "withdraw"
)

// Outcome of a school in a preview: ready to take the version or refused.
const (
	PreviewReady    = "ready"
	PreviewRejected = "rejected"
)

// OfferingPlan is one Träger offering at one school.
type OfferingPlan struct {
	TraegerOfferingID int64                         `json:"traeger_offering_id"`
	Name              string                        `json:"name"`
	Action            string                        `json:"action"`
	Fields            []careplan.TraegerFieldChange `json:"fields"`
	// PricesFrom is the month a new tariff of the school starts, nil when
	// the prices stay.
	PricesFrom *calendar.Date `json:"prices_from,omitempty"`
	// ContractTemplate reports that the school's contract template is
	// written.
	ContractTemplate bool `json:"contract_template"`
}

// SchoolPlan is what a rollout does at one school, or why it refuses.
type SchoolPlan struct {
	SchoolID  int64                           `json:"school_id"`
	Action    string                          `json:"action"`
	Outcome   string                          `json:"outcome"`
	ErrorCode string                          `json:"error_code,omitempty"`
	Phase     []enrollment.TraegerFieldChange `json:"phase"`
	Offerings []OfferingPlan                  `json:"offerings"`
}

// Preview is the plan of a rollout per school with its fingerprint.
type Preview struct {
	TraegerPhaseID int64        `json:"traeger_phase_id"`
	Version        int          `json:"version"`
	Fingerprint    string       `json:"fingerprint"`
	Schools        []SchoolPlan `json:"schools"`
}

// SchoolResult is the outcome of a rollout at one school.
type SchoolResult struct {
	SchoolID  int64  `json:"school_id"`
	Outcome   string `json:"outcome"`
	ErrorCode string `json:"error_code,omitempty"`
}

// Rollout is the outcome of a rollout per school. A rollout is atomic per
// school, not across schools.
type Rollout struct {
	TraegerPhaseID int64          `json:"traeger_phase_id"`
	Version        int            `json:"version"`
	Schools        []SchoolResult `json:"schools"`
}

// Catalog previews and rolls out a Träger phase. Each call opens the
// Träger transaction and the schools' tenant transactions itself, one after
// the other and never nested: the caller holds no transaction.
type Catalog interface {
	Preview(ctx context.Context, actor Actor, traegerPhaseID int64) (Preview, error)
	Rollout(ctx context.Context, actor Actor, traegerPhaseID int64, fingerprint string) (Rollout, error)
}

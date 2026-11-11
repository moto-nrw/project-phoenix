// Package ports declares the consumer-owned ports of the Träger catalog
// rollout (ADR 0195 section 3); the root binds them to the owners.
package ports

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Schools lists the undeleted schools of a Träger (Organisation & Tenancy).
type Schools interface {
	ListTraegerSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error)
}

// PhaseTemplates are Enrollment's Träger phases, in the Träger transaction.
type PhaseTemplates interface {
	TraegerPhase(ctx context.Context, organizationID, id int64) (enrollment.TraegerPhase, error)
	LockTraegerPhase(ctx context.Context, organizationID, id int64) (enrollment.TraegerPhase, error)
	ReleaseTraegerPhase(ctx context.Context, organizationID, id, accountID int64) (int, error)
	RecordTraegerPhaseOutcomes(ctx context.Context, organizationID, id int64, outcomes []enrollment.TraegerCopyOutcome) error
}

// OfferingTemplates are Care Plan's Träger offerings, in the Träger
// transaction.
type OfferingTemplates interface {
	ListTraegerOfferings(ctx context.Context, organizationID, traegerPhaseID int64) ([]careplan.TraegerOffering, error)
	ReleaseTraegerOfferings(ctx context.Context, organizationID, traegerPhaseID, accountID int64) ([]careplan.TraegerOffering, error)
	RecordTraegerOfferingOutcomes(ctx context.Context, organizationID, id int64, outcomes []careplan.TraegerOfferingOutcome) error
}

// SchoolPhases copy a Träger phase in one school's tenant transaction.
type SchoolPhases = enrollment.TraegerPhaseCopies

// SchoolOfferings copy Träger offerings in one school's tenant transaction.
type SchoolOfferings = careplan.TraegerOfferingCopies

// ModulePrices are the prices of one module of a school from a month on.
type ModulePrices struct {
	SchoolID       int64
	CareOfferingID int64
	ModuleName     string
	ValidFrom      calendar.Date
	Prices         []careplan.TraegerCarePrice
}

// Tariffs write a school's prices of a module in Parent Billing, in the
// Träger transaction.
type Tariffs func(ctx context.Context, organizationID, accountID int64, prices ModulePrices) error

// ContractTemplates write a school's contract template of a module in Care
// Contract. The owner opens its own transactions; the caller holds none.
type ContractTemplates func(ctx context.Context, organizationID, accountID, schoolID, careOfferingID int64, template careplan.TraegerContractTemplate) error

// Runtime opens the Träger transaction or one school's tenant transaction,
// never both at once (ADR 0105 point 2).
type Runtime interface {
	WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error
	WithinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error
}

// Dependencies bind the rollout to its owners.
type Dependencies struct {
	Schools           Schools
	Phases            PhaseTemplates
	Offerings         OfferingTemplates
	SchoolPhases      SchoolPhases
	SchoolOfferings   SchoolOfferings
	Tariffs           Tariffs
	ContractTemplates ContractTemplates
	Runtime           Runtime
	Today             func() calendar.Date
	// Fingerprint is the SHA-256 hex digest of content (Security Runtime).
	Fingerprint func(content []byte) string
}

// Refusal is an owner's refusal of a school's tariff or contract template
// with its registered code; the binding marks it so the rollout records it
// for that school.
type Refusal struct {
	Code string
	Err  error
}

func (r *Refusal) Error() string { return fmt.Sprintf("%s: %v", r.Code, r.Err) }

// Unwrap returns the owner's error.
func (r *Refusal) Unwrap() error { return r.Err }

// ErrorCode is the registered code of the refusal.
func (r *Refusal) ErrorCode() string { return r.Code }

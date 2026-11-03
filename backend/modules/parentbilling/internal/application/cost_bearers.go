package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The cost bearers of a Träger (#5627): the Abrechnung keeps them with its
// contribution rules, as ADR 0088 section 4 names the cost bearer a payer of
// the chain. A cost bearer that a frozen contribution or an invoice names
// can only be switched off.

// CostBearerStore persists the cost bearers of a Träger in its Träger
// transaction.
type CostBearerStore interface {
	ListCostBearers(ctx context.Context, organizationID int64) ([]parentbilling.CostBearer, error)
	// CostBearer reads one cost bearer; a missing one is
	// ErrCostBearerNotFound.
	CostBearer(ctx context.Context, organizationID, costBearerID int64) (parentbilling.CostBearer, error)
	// LockCostBearer reads one cost bearer and holds its row until the
	// transaction ends, shared for a confirmation, exclusive for a
	// deletion; a missing one is ErrCostBearerNotFound.
	LockCostBearer(ctx context.Context, organizationID, costBearerID int64, exclusive bool) (parentbilling.CostBearer, error)
	// CostBearersInUse names the cost bearers a frozen contribution line or
	// an invoice position names.
	CostBearersInUse(ctx context.Context, organizationID int64) (map[int64]bool, error)
	InsertCostBearer(ctx context.Context, organizationID int64, input parentbilling.CostBearerInput) (parentbilling.CostBearer, error)
	// UpdateCostBearer reports a missing cost bearer as
	// ErrCostBearerNotFound.
	UpdateCostBearer(ctx context.Context, organizationID, costBearerID int64, input parentbilling.CostBearerInput) (parentbilling.CostBearer, error)
	// DeleteCostBearer deletes a cost bearer no line and no position names;
	// a missing one is ErrCostBearerNotFound, one named in the meantime
	// ErrCostBearerInUse.
	DeleteCostBearer(ctx context.Context, organizationID, costBearerID int64) error
}

// ListCostBearers lists the cost bearers of the Träger by name and marks
// those a frozen contribution or an invoice names.
func (s *Rules) ListCostBearers(ctx context.Context, organizationID int64) ([]parentbilling.CostBearer, error) {
	if organizationID <= 0 {
		return nil, errTraegerRequired
	}
	bearers, err := s.store.ListCostBearers(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: list cost bearers: %w", err)
	}
	inUse, err := s.store.CostBearersInUse(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read cost bearers in use: %w", err)
	}
	for i := range bearers {
		bearers[i].InUse = inUse[bearers[i].ID]
	}
	return bearers, nil
}

// SaveCostBearer validates a cost bearer and creates it, or changes the one
// costBearerID names. A frozen contribution and an issued invoice keep the
// name they copied.
func (s *Rules) SaveCostBearer(ctx context.Context, organizationID, costBearerID int64, input parentbilling.CostBearerInput) (parentbilling.CostBearer, error) {
	if organizationID <= 0 {
		return parentbilling.CostBearer{}, errTraegerRequired
	}
	if costBearerID < 0 {
		return parentbilling.CostBearer{}, parentbilling.ErrCostBearerNotFound
	}
	input, err := normalizeCostBearer(input)
	if err != nil {
		return parentbilling.CostBearer{}, err
	}
	if costBearerID == 0 {
		return s.store.InsertCostBearer(ctx, organizationID, input)
	}
	return s.store.UpdateCostBearer(ctx, organizationID, costBearerID, input)
}

// DeleteCostBearer removes a cost bearer nothing names; a named one stays
// for the contributions and invoices that copied it and can only be
// switched off.
func (s *Rules) DeleteCostBearer(ctx context.Context, organizationID, costBearerID int64) error {
	if organizationID <= 0 {
		return errTraegerRequired
	}
	if costBearerID <= 0 {
		return parentbilling.ErrCostBearerNotFound
	}
	// The exclusive lock waits for a running confirmation with this cost
	// bearer, so the check below sees the lines it wrote.
	if _, err := s.store.LockCostBearer(ctx, organizationID, costBearerID, true); err != nil {
		return err
	}
	inUse, err := s.store.CostBearersInUse(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("parent billing: read cost bearers in use: %w", err)
	}
	if inUse[costBearerID] {
		return parentbilling.ErrCostBearerInUse
	}
	return s.store.DeleteCostBearer(ctx, organizationID, costBearerID)
}

// normalizeCostBearer trims the fields and checks them: a name on one line,
// an address of at most a few lines, a file reference on one line.
func normalizeCostBearer(input parentbilling.CostBearerInput) (parentbilling.CostBearerInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Address = strings.TrimSpace(strings.ReplaceAll(input.Address, "\r\n", "\n"))
	input.FileReference = strings.TrimSpace(input.FileReference)
	errs := validation.Errors{}
	switch {
	case input.Name == "":
		errs["name"] = errors.New("is required")
	case len([]rune(input.Name)) > parentbilling.MaxCostBearerNameLength || strings.ContainsAny(input.Name, "\r\n"):
		errs["name"] = errors.New("must be one line of at most 200 characters")
	}
	if len([]rune(input.Address)) > parentbilling.MaxCostBearerAddressLength {
		errs["address"] = errors.New("must be at most 500 characters")
	}
	if len([]rune(input.FileReference)) > parentbilling.MaxCostBearerFileReferenceLength || strings.ContainsAny(input.FileReference, "\r\n") {
		errs["file_reference"] = errors.New("must be one line of at most 100 characters")
	}
	if len(errs) > 0 {
		return input, errs
	}
	return input, nil
}

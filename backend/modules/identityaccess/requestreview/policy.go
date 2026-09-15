// Package requestreview is the Identity & Access review-scope capability.
// It evaluates request identity and tenant setting facts, never JWTs or models.
package requestreview

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

var ErrAbsenceReadRequired = errors.New("the users:read permission is required alongside users:absence")

// Principal contains effective permission facts resolved at the inbound
// identity boundary, including wildcard permissions.
type Principal struct {
	Admin        bool
	UsersUpdate  bool
	UsersRead    bool
	UsersAbsence bool
}

type Scope struct {
	SchoolWide bool
	GroupIDs   []int64
}

type Dependencies struct {
	Principal          func(context.Context) Principal
	GroupLeaderEnabled func(context.Context) (bool, error)
	GroupIDs           func(context.Context) ([]int64, error)
}

type Policy struct{ deps Dependencies }

func New(deps Dependencies) (*Policy, error) {
	if deps.Principal == nil || deps.GroupLeaderEnabled == nil || deps.GroupIDs == nil {
		return nil, errors.New("parent request review policy is not configured")
	}
	return &Policy{deps: deps}, nil
}

func (p *Policy) Principal(ctx context.Context) Principal {
	return p.deps.Principal(ctx)
}

func (p *Policy) Scope(ctx context.Context) (Scope, error) {
	principal := p.Principal(ctx)
	if principal.Admin {
		return Scope{SchoolWide: true}, nil
	}
	if !principal.UsersUpdate {
		if principal.UsersAbsence && !principal.UsersRead {
			return Scope{}, ErrAbsenceReadRequired
		}
		if !principal.UsersAbsence || !principal.UsersRead {
			return Scope{}, nil
		}
	}
	enabled, err := p.deps.GroupLeaderEnabled(ctx)
	if err != nil {
		return Scope{}, fmt.Errorf("resolve group-leader request review setting: %w", err)
	}
	if !enabled {
		return Scope{}, nil
	}
	groups, err := p.deps.GroupIDs(ctx)
	if err != nil {
		return Scope{}, fmt.Errorf("resolve request review groups: %w", err)
	}
	ids := make([]int64, 0, len(groups))
	for _, id := range groups {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return Scope{GroupIDs: slices.Compact(ids)}, nil
}

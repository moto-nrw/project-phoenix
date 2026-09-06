package config

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// BlockPolicy is what a school prescribes for one start page block.
type BlockPolicy string

const (
	// BlockOptional lets each person decide. This is the state of every block
	// the school has not spoken about, so it is never stored.
	BlockOptional BlockPolicy = "optional"
	// BlockRequired shows the block to everybody who may see it. The person can
	// still not be shown a block they lack the permission for.
	BlockRequired BlockPolicy = "required"
	// BlockDisabled removes the block from the school entirely: nobody sees it
	// and it does not appear in the customize dialog.
	BlockDisabled BlockPolicy = "disabled"
)

// ParseBlockPolicy accepts the three stored spellings and nothing else.
func ParseBlockPolicy(raw string) (BlockPolicy, error) {
	switch BlockPolicy(raw) {
	case BlockOptional, BlockRequired, BlockDisabled:
		return BlockPolicy(raw), nil
	default:
		return "", fmt.Errorf("unknown block policy %q", raw)
	}
}

// homeBlockKeyPattern is the whole validation a block key gets.
//
// The catalogue of start page blocks lives in the frontend: it is the only
// layer that knows a block's label, the permission behind it and the operating
// modes it makes sense in. Pinning a copy of that catalogue here would buy
// nothing — neither store is a permission boundary, every block fetches its
// data from an endpoint that checks permissions itself — and would cost a
// backend deploy for every new block. So the API checks the shape and the size,
// which is what keeps junk out of the column.
var homeBlockKeyPattern = regexp.MustCompile(`^(tile|section)\.[a-z0-9_]{1,48}$`)

// MaxHomeBlockEntries caps one stored map. The start page has well under 50
// blocks; the cap exists so a client cannot grow the row without bound.
const MaxHomeBlockEntries = 100

// ValidateHomeBlockKey rejects anything that is not a well-formed block key.
func ValidateHomeBlockKey(key string) error {
	if !homeBlockKeyPattern.MatchString(key) {
		return fmt.Errorf("invalid start page block key %q", key)
	}
	return nil
}

// HomeLayout is one person's start page at one school: only their deviations
// from what their role and the school recommend.
//
// A block that is absent from Overrides is not "hidden", it is "undecided" and
// falls back to the recommendation. That distinction is the whole point of the
// table: a block introduced in a later release reaches existing accounts in its
// intended default state instead of disappearing for everybody who ever opened
// the dialog.
type HomeLayout struct {
	ID        int64           `bun:"id,pk,autoincrement" json:"id"`
	TenantID  int64           `bun:"tenant_id,notnull" json:"tenant_id"`
	AccountID int64           `bun:"account_id,notnull" json:"account_id"`
	Overrides map[string]bool `bun:"overrides,type:jsonb,notnull" json:"overrides"`
	CreatedAt time.Time       `bun:"created_at,notnull,default:now()" json:"created_at"`
	UpdatedAt time.Time       `bun:"updated_at,notnull,default:now()" json:"updated_at"`
}

func (l *HomeLayout) GetTenantID() int64   { return l.TenantID }
func (l *HomeLayout) SetTenantID(id int64) { l.TenantID = id }

// Validate checks the row identifies an account and carries a sane map.
func (l *HomeLayout) Validate() error {
	if l.AccountID <= 0 {
		return errors.New("account ID is required")
	}
	if len(l.Overrides) > MaxHomeBlockEntries {
		return fmt.Errorf("at most %d start page blocks can be stored", MaxHomeBlockEntries)
	}
	for key := range l.Overrides {
		if err := ValidateHomeBlockKey(key); err != nil {
			return err
		}
	}
	return nil
}

// HomeBlockPolicySet is what one school prescribes for its start page.
//
// Like HomeLayout it stores only deviations: a block absent from Policies is
// BlockOptional, which is also the state of every block for a school that has
// never opened the dialog.
type HomeBlockPolicySet struct {
	ID        int64                  `bun:"id,pk,autoincrement" json:"id"`
	TenantID  int64                  `bun:"tenant_id,notnull" json:"tenant_id"`
	Policies  map[string]BlockPolicy `bun:"policies,type:jsonb,notnull" json:"policies"`
	UpdatedBy *int64                 `bun:"updated_by" json:"updated_by,omitempty"`
	CreatedAt time.Time              `bun:"created_at,notnull,default:now()" json:"created_at"`
	UpdatedAt time.Time              `bun:"updated_at,notnull,default:now()" json:"updated_at"`
}

func (p *HomeBlockPolicySet) GetTenantID() int64   { return p.TenantID }
func (p *HomeBlockPolicySet) SetTenantID(id int64) { p.TenantID = id }

// Validate checks the prescribed map is well-formed.
func (p *HomeBlockPolicySet) Validate() error {
	if len(p.Policies) > MaxHomeBlockEntries {
		return fmt.Errorf("at most %d start page blocks can be stored", MaxHomeBlockEntries)
	}
	for key, policy := range p.Policies {
		if err := ValidateHomeBlockKey(key); err != nil {
			return err
		}
		if _, err := ParseBlockPolicy(string(policy)); err != nil {
			return err
		}
	}
	return nil
}

// HomeLayoutRepository stores personal start page composition and the school's
// prescription for it. Both live behind one interface because the start page
// always reads them together and never one without the other.
type HomeLayoutRepository interface {
	// FindByAccount returns one account's stored deviations in the current
	// tenant, or (nil, nil) when the account has never customized anything.
	FindByAccount(ctx context.Context, accountID int64) (*HomeLayout, error)

	// UpsertForAccount replaces an account's deviations wholesale. The map is
	// always written as a whole; there is no partial update.
	UpsertForAccount(ctx context.Context, layout *HomeLayout) error

	// DeleteForAccount drops the stored deviations, which restores the
	// recommended start page.
	DeleteForAccount(ctx context.Context, accountID int64) error

	// FindPolicies returns the current tenant's prescription, or (nil, nil)
	// when the school has never prescribed anything.
	FindPolicies(ctx context.Context) (*HomeBlockPolicySet, error)

	// UpsertPolicies replaces the tenant's prescription wholesale.
	UpsertPolicies(ctx context.Context, policies *HomeBlockPolicySet) error
}

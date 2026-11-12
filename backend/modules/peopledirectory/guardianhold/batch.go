package guardianhold

import (
	"context"
	"slices"
	"strings"
)

// BatchLocks adds set-based child and relationship locks to the same
// identity/profile protocol Hold uses. Relationships returns grant checks
// only for links that still exist after their rows have been locked.
type BatchLocks struct {
	Locks
	Children      func(context.Context, []int64) error
	Relationships func(context.Context, []int64, int64) (map[int64]func(string) bool, error)
}

// HoldMany holds one identity's authority over the candidates until commit.
// Membership and profile checks run once, children and relationships are
// locked as sets in ascending student order, before evaluating each grant.
func HoldMany(ctx context.Context, locks BatchLocks, req Request, studentIDs []int64) (map[int64]bool, error) {
	ids := slices.Clone(studentIDs)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) == 0 {
		return map[int64]bool{}, nil
	}
	if ids[0] <= 0 || req.ProfileID <= 0 || (req.AccountID <= 0 && strings.TrimSpace(req.Email) == "") {
		return nil, ErrNotLinked
	}
	if locks.Membership == nil || locks.Children == nil || locks.Profile == nil || locks.Relationships == nil || locks.Owner == nil {
		return nil, ErrUnavailable
	}
	var links map[int64]func(string) bool
	err := holdAuthority(ctx, locks.Locks, req, func(ctx context.Context) error {
		return locks.Children(ctx, ids)
	}, func(ctx context.Context) error {
		var err error
		links, err = locks.Relationships(ctx, ids, req.ProfileID)
		return err
	})
	if err != nil {
		return nil, err
	}
	grants := make(map[int64]bool, len(ids))
	for _, id := range ids {
		granted := links[id]
		grants[id] = granted != nil && requirePermissions(granted, req) == nil
	}
	return grants, nil
}

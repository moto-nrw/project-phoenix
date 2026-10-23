package application

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type RequestShareRecipients struct {
	shares     peopledirectory.ParentRequestShares
	protection peopledirectory.GuardianFamilyProtectionReads
}

func NewRequestShareRecipients(shares peopledirectory.ParentRequestShares, protection peopledirectory.GuardianFamilyProtectionReads) (*RequestShareRecipients, error) {
	if shares == nil || protection == nil {
		return nil, errors.New("parent: request share visibility needs the sharing ledger and the family protection")
	}
	return &RequestShareRecipients{shares: shares, protection: protection}, nil
}

func (r *RequestShareRecipients) SharedRecipientAccountIDs(ctx context.Context, studentID int64, requestType string, requestID int64) ([]int64, error) {
	shares, err := r.shares.ParentRequestSharesForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	protection, err := r.protection.GuardianFamilyProtectionRecords(ctx, []int64{studentID})
	if err != nil {
		return nil, err
	}
	var protected bool
	var protectedAt time.Time
	if event := protection[studentID]; event != nil {
		protected, protectedAt = event.Enabled, event.CreatedAt
	}
	ids := make([]int64, 0)
	for _, share := range shares {
		if share.RequestType != requestType || share.RequestID != requestID {
			continue
		}
		for id := range (peopledirectory.RequestSharePolicy{}).EffectiveRecipientSet(share.CreatedAt, share.RecipientAccountIDs, protected, protectedAt) {
			if id > 0 {
				ids = append(ids, id)
			}
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

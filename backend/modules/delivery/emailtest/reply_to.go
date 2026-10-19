package emailtest

import (
	"context"

	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
)

// ReplyToResolverStub records the tenant lookup and returns a configured identity.
type ReplyToResolverStub struct {
	Name, Address string
	TenantID      int64
	Err           error
}

func (s *ReplyToResolverStub) ResolveReplyTo(_ context.Context, tenantID int64) (email.ReplyToIdentity, error) {
	s.TenantID = tenantID
	return email.ReplyToIdentity{Name: s.Name, Address: s.Address}, s.Err
}

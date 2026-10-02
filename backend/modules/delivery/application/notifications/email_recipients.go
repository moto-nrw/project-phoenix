package notifications

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Opt-in e-mail to staff (#3780).
//
// An e-mail type reaches every account of the school that switched it on and
// still holds the type's permission when the mail is sent. The decision is the
// same consent row as a push type's (PreferenceService.SetEmailSubscribed);
// only the catalogue entry differs (ChannelEmail). A producer resolves the
// addresses here and enqueues one outbox mail per address itself, because the
// mail kind, its payload and its renderer belong to the producer.

// permissionConfigManage restates the permission registry's name, which this
// package may not import; TestEmailTypeCatalogue pins it to it.
const permissionConfigManage = "config:manage"

// The opt-in e-mail types. Each is decided where its event happens, never on
// the profile page, so it needs no description or heading there.
func init() {
	// Decided in the Anmeldungen overview. Only accounts that may manage
	// enrollments receive it.
	RegisterType(TypeDefinition{
		Key:        TypeEnrollmentSubmitted,
		Label:      "Neue Anmeldung",
		Portal:     PortalStaff,
		Channel:    ChannelEmail,
		Permission: permissionConfigManage,
	})
}

// EmailSubscribers is the slice of Communication's consent capability the
// resolver needs: every account of the current school that switched a type
// on. It is only the candidate set; Recipients narrows it by membership and
// permission before anybody is addressed.
type EmailSubscribers interface {
	ListOptedIn(ctx context.Context, notificationType string) ([]int64, error)
}

// EmailAccounts is the slice of Identity & Access the resolver reads: active
// membership and effective permissions at one school, and the login addresses
// of the remaining accounts.
type EmailAccounts interface {
	ListActiveAccountIDsForTenant(ctx context.Context, tenantID int64, accountIDs []int64) ([]int64, error)
	FindEffectivePermissionNamesByAccountIDsForTenant(ctx context.Context, accountIDs []int64, tenantID int64) (map[int64][]string, error)
	ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error)
}

// EmailRecipientResolver resolves the addresses of one opt-in e-mail.
type EmailRecipientResolver interface {
	// Recipients returns the addresses one mail of the type goes to in the
	// school in context: additional first, then every account that switched
	// the type on, is an active member and holds the permission. Addresses
	// are trimmed and deduplicated case-insensitively.
	Recipients(ctx context.Context, notificationType string, additional []string) ([]string, error)
}

type emailRecipientResolver struct {
	subscribers EmailSubscribers
	accounts    EmailAccounts
}

// NewEmailRecipientResolver builds the resolver for opt-in e-mail types.
func NewEmailRecipientResolver(subscribers EmailSubscribers, accounts EmailAccounts) EmailRecipientResolver {
	return &emailRecipientResolver{subscribers: subscribers, accounts: accounts}
}

func (s *emailRecipientResolver) Recipients(ctx context.Context, notificationType string, additional []string) ([]string, error) {
	def, ok := EmailType(notificationType)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownNotificationType, notificationType)
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, errors.New("e-mail recipients need a school in context")
	}

	addresses := newAddressSet()
	for _, address := range additional {
		addresses.add(address)
	}

	candidates, err := s.subscribers.ListOptedIn(ctx, notificationType)
	if err != nil {
		return nil, fmt.Errorf("list e-mail subscribers: %w", err)
	}
	if len(candidates) == 0 {
		return addresses.list, nil
	}
	active, err := s.accounts.ListActiveAccountIDsForTenant(ctx, tenantID, candidates)
	if err != nil {
		return nil, fmt.Errorf("check e-mail subscriber memberships: %w", err)
	}
	if len(active) == 0 {
		return addresses.list, nil
	}
	names, err := s.accounts.FindEffectivePermissionNamesByAccountIDsForTenant(ctx, active, tenantID)
	if err != nil {
		return nil, fmt.Errorf("check e-mail subscriber permissions: %w", err)
	}
	permitted := make([]int64, 0, len(active))
	for _, accountID := range active {
		if authorize.HasPermission(def.Permission, names[accountID]) {
			permitted = append(permitted, accountID)
		}
	}
	if len(permitted) == 0 {
		return addresses.list, nil
	}
	slices.Sort(permitted)
	emails, err := s.accounts.ListAccountEmails(ctx, permitted)
	if err != nil {
		return nil, fmt.Errorf("load e-mail subscriber addresses: %w", err)
	}
	for _, accountID := range permitted {
		addresses.add(emails[accountID])
	}
	return addresses.list, nil
}

// addressSet keeps the first spelling of every address, in insertion order.
type addressSet struct {
	seen map[string]struct{}
	list []string
}

func newAddressSet() *addressSet {
	return &addressSet{seen: make(map[string]struct{})}
}

func (s *addressSet) add(address string) {
	trimmed := strings.TrimSpace(address)
	if trimmed == "" {
		return
	}
	key := strings.ToLower(trimmed)
	if _, dup := s.seen[key]; dup {
		return
	}
	s.seen[key] = struct{}{}
	s.list = append(s.list, trimmed)
}

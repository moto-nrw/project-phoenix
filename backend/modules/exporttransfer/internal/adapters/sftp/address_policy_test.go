package sftp

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The outbound address policy (#3050) is the SSRF guard. Whoever can edit the
// school settings decides which host this server connects to, so the policy
// is the only thing between a settings field and the internal network.

func TestPublicOnlyPolicy_RejectsNonPublicAddresses(t *testing.T) {
	t.Parallel()

	blocked := map[string]string{
		"IPv4 loopback":          "127.0.0.1",
		"IPv4 loopback range":    "127.99.42.7",
		"private 10/8":           "10.0.0.5",
		"private 172.16/12":      "172.16.4.9",
		"private 192.168/16":     "192.168.1.10",
		"link-local (metadata)":  "169.254.169.254",
		"CGNAT":                  "100.64.1.1",
		"unspecified":            "0.0.0.0",
		"broadcast":              "255.255.255.255",
		"multicast":              "224.0.0.1",
		"TEST-NET-1":             "192.0.2.1",
		"TEST-NET-3":             "203.0.113.9",
		"benchmarking":           "198.18.0.1",
		"IPv6 loopback":          "::1",
		"IPv6 unique local":      "fd00::1",
		"IPv6 link-local":        "fe80::1",
		"IPv6 unspecified":       "::",
		"IPv6 documentation":     "2001:db8::1",
		"IPv4-mapped private":    "::ffff:192.168.1.10",
		"IPv4-mapped loopback":   "::ffff:127.0.0.1",
		"IPv4-mapped link-local": "::ffff:169.254.169.254",
	}
	policy := PublicOnlyPolicy{}
	for name, raw := range blocked {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			addr := netip.MustParseAddr(raw)
			assert.Falsef(t, policy.Allow(addr), "%s (%s) must not be a transfer target", name, raw)
		})
	}
}

func TestPublicOnlyPolicy_AllowsPublicAddresses(t *testing.T) {
	t.Parallel()

	allowed := map[string]string{
		"public IPv4":       "93.184.216.34",
		"public IPv4 other": "8.8.8.8",
		"public IPv6":       "2606:2800:220:1:248:1893:25c8:1946",
	}
	policy := PublicOnlyPolicy{}
	for name, raw := range allowed {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Truef(t, policy.Allow(netip.MustParseAddr(raw)), "%s (%s) should be reachable", name, raw)
		})
	}
}

func TestPublicOnlyPolicy_RejectsTheZeroAddress(t *testing.T) {
	t.Parallel()

	assert.False(t, PublicOnlyPolicy{}.Allow(netip.Addr{}))
}

// A literal IP in the settings must go through the same policy as a hostname
// — otherwise the guard is bypassed by simply typing the address.
func TestResolveAllowedAddr_AppliesThePolicyToLiteralAddresses(t *testing.T) {
	t.Parallel()

	unreachableResolver := func(context.Context, string) ([]netip.Addr, error) {
		t.Fatal("a literal address must not be resolved")
		return nil, nil
	}

	_, err := resolveAllowedAddr(context.Background(), unreachableResolver, PublicOnlyPolicy{}, "127.0.0.1")
	require.ErrorIs(t, err, ErrAddressNotAllowed)

	addr, err := resolveAllowedAddr(context.Background(), unreachableResolver, PublicOnlyPolicy{}, "93.184.216.34")
	require.NoError(t, err)
	assert.Equal(t, "93.184.216.34", addr.String())
}

// DNS rebinding: a name that answers with both a public and a private address
// is refused outright. Picking the public half would let the counterpart
// decide which answer the actual connection receives.
func TestResolveAllowedAddr_RejectsMixedPublicAndPrivateAnswers(t *testing.T) {
	t.Parallel()

	resolver := func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{
			netip.MustParseAddr("93.184.216.34"),
			netip.MustParseAddr("169.254.169.254"),
		}, nil
	}

	_, err := resolveAllowedAddr(context.Background(), resolver, PublicOnlyPolicy{}, "rebind.beispiel.de")
	require.ErrorIs(t, err, ErrAddressNotAllowed)
}

func TestResolveAllowedAddr_ReturnsAnAddressNotAHostname(t *testing.T) {
	t.Parallel()

	resolver := func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	}

	addr, err := resolveAllowedAddr(context.Background(), resolver, PublicOnlyPolicy{}, "dateien.beispiel.de")
	require.NoError(t, err)
	// The caller dials THIS, so a second lookup cannot substitute a different
	// address after the check.
	assert.Equal(t, "93.184.216.34", addr.String())
}

func TestResolveAllowedAddr_PropagatesResolutionFailure(t *testing.T) {
	t.Parallel()

	boom := errors.New("no such host")
	resolver := func(context.Context, string) ([]netip.Addr, error) { return nil, boom }

	_, err := resolveAllowedAddr(context.Background(), resolver, PublicOnlyPolicy{}, "gibt-es-nicht.beispiel.de")
	require.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, ErrAddressNotAllowed, "a lookup failure is not a policy verdict")
}

func TestResolveAllowedAddr_RejectsAnEmptyAnswer(t *testing.T) {
	t.Parallel()

	resolver := func(context.Context, string) ([]netip.Addr, error) { return nil, nil }

	_, err := resolveAllowedAddr(context.Background(), resolver, PublicOnlyPolicy{}, "leer.beispiel.de")
	require.Error(t, err)
}

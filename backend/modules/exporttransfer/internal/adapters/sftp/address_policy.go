package sftp

import (
	"context"
	"fmt"
	"net"
	"net/netip"
)

// Outbound address policy (#3050).
//
// The SFTP transfer is the first feature where a school administrator's
// settings decide which host this server opens a connection to. Without a
// guard that is a server-side request forgery primitive: whoever can edit the
// settings could point the transfer at 127.0.0.1, at the database, at a cloud
// metadata endpoint, and read the outcome from the error message.
//
// So the first expansion stage allows PUBLIC destinations only. That is a
// deliberate product limitation, not a technical placeholder: a school with a
// counterpart inside its own network cannot use the feature yet.

// ErrAddressNotAllowed marks a destination outside the permitted address
// space. It is deliberately coarse — the caller turns it into one German
// sentence and must not report back which range matched, since that answer is
// itself the scan result an attacker is after.
var ErrAddressNotAllowed = reason{"address not allowed", "address_denied"}

// AddressPolicy decides whether a resolved IP may be dialed.
type AddressPolicy interface {
	Allow(addr netip.Addr) bool
}

// PublicOnlyPolicy permits globally routable unicast addresses only.
type PublicOnlyPolicy struct{}

func (PublicOnlyPolicy) Allow(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	// Unwrap ::ffff:a.b.c.d so an IPv4 range cannot be smuggled past the
	// checks below in IPv6 clothing.
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	switch {
	case addr.IsLoopback(),
		addr.IsPrivate(),
		addr.IsLinkLocalUnicast(),
		addr.IsLinkLocalMulticast(),
		addr.IsMulticast(),
		addr.IsUnspecified(),
		addr.IsInterfaceLocalMulticast():
		return false
	}
	// Ranges netip has no predicate for but which are just as much "not the
	// public internet".
	for _, prefix := range reservedPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

// reservedPrefixes covers the special-purpose ranges that are neither private
// nor loopback by netip's definition, yet must never be a transfer target.
var reservedPrefixes = buildReservedPrefixes()

func buildReservedPrefixes() []netip.Prefix {
	raw := []string{
		"100.64.0.0/10",   // CGNAT — carrier-internal, reachable from some hosts
		"192.0.0.0/24",    // IETF protocol assignments
		"192.0.2.0/24",    // TEST-NET-1
		"198.18.0.0/15",   // benchmarking
		"198.51.100.0/24", // TEST-NET-2
		"203.0.113.0/24",  // TEST-NET-3
		"240.0.0.0/4",     // reserved, includes 255.255.255.255
		"::/128",          // unspecified
		"64:ff9b::/96",    // IPv4/IPv6 translation
		"100::/64",        // discard-only
		"2001:db8::/32",   // documentation
		"fc00::/7",        // unique local
	}
	prefixes := make([]netip.Prefix, 0, len(raw))
	for _, entry := range raw {
		// The list is a compile-time constant; a typo in it is a programming
		// error, not a runtime condition.
		prefixes = append(prefixes, netip.MustParsePrefix(entry))
	}
	return prefixes
}

// Resolver looks a hostname up. Injected so tests can point a host name at a
// loopback test server without switching the policy off.
type Resolver func(ctx context.Context, host string) ([]netip.Addr, error)

// DefaultResolver resolves through the system resolver.
func DefaultResolver(ctx context.Context, host string) ([]netip.Addr, error) {
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}
	return ips, nil
}

// resolveAllowedAddr resolves the host and returns ONE address that may be
// dialed.
//
// Two rules matter here, and both exist to close DNS rebinding:
//
//  1. EVERY resolved address must pass the policy. A name that answers with a
//     public and a private address is rejected outright rather than dialed on
//     its public half — the far side chooses which answer arrives.
//  2. The caller dials the returned ADDRESS, never the hostname again. A
//     second lookup could return something else than the one just checked.
func resolveAllowedAddr(ctx context.Context, resolve Resolver, policy AddressPolicy, host string) (netip.Addr, error) {
	// A literal IP in the settings skips name resolution but not the policy.
	if literal, err := netip.ParseAddr(host); err == nil {
		if !policy.Allow(literal) {
			return netip.Addr{}, ErrAddressNotAllowed
		}
		return literal, nil
	}

	addrs, err := resolve(ctx, host)
	if err != nil {
		return netip.Addr{}, err
	}
	if len(addrs) == 0 {
		return netip.Addr{}, fmt.Errorf("resolve %s: no addresses", host)
	}
	for _, addr := range addrs {
		if !policy.Allow(addr) {
			return netip.Addr{}, ErrAddressNotAllowed
		}
	}
	return addrs[0], nil
}

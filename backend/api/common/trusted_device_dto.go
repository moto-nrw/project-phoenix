package common

import (
	"net"
	"time"
)

// TrustedDeviceDTO is the wire shape for an MFA trusted-device row.
// The token hash stays server-side; we surface only the columns the
// user can act on: id (for revoke), user-agent + IP for "is this
// still you", and the three timestamps the list UI consumes
// ("created on ..., expires ..., last used ...").
//
// Shared between the tenant /auth/mfa/trusted-devices endpoints and
// the operator /operator/auth/mfa/trusted-devices endpoints —
// extracted to remove the duplicated DTO definition and mapping
// loop flagged by SonarCloud.
type TrustedDeviceDTO struct {
	ID         int64   `json:"id"`
	UserAgent  *string `json:"user_agent,omitempty"`
	IPAddress  string  `json:"ip_address,omitempty"`
	CreatedAt  string  `json:"created_at"`
	ExpiresAt  string  `json:"expires_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
}

// TrustedDeviceRow is the minimal projection of a trusted-device model
// row needed to build a TrustedDeviceDTO. Both the tenant-scoped
// auth.MFATrustedDevice and the operator-scoped
// platform.OperatorMFATrustedDevice expose these fields, so the same
// mapping helper can serve both endpoints without duplicating the loop.
type TrustedDeviceRow struct {
	ID         int64
	UserAgent  *string
	IPAddress  net.IP
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastUsedAt *time.Time
}

// NewTrustedDeviceDTO formats a single row for the wire, normalising
// timestamps to RFC3339-UTC and omitting the optional IP / last-used
// fields when the source values are nil.
func NewTrustedDeviceDTO(row TrustedDeviceRow) TrustedDeviceDTO {
	dto := TrustedDeviceDTO{
		ID:        row.ID,
		UserAgent: row.UserAgent,
		CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339),
		ExpiresAt: row.ExpiresAt.UTC().Format(time.RFC3339),
	}
	if row.IPAddress != nil {
		dto.IPAddress = row.IPAddress.String()
	}
	if row.LastUsedAt != nil {
		s := row.LastUsedAt.UTC().Format(time.RFC3339)
		dto.LastUsedAt = &s
	}
	return dto
}

// MapTrustedDevices projects a slice of concrete model rows into DTOs
// in declaration order. The projection function turns one model row
// into a TrustedDeviceRow — typically a one-liner that copies fields
// of the same name. This generic shape lets the tenant-side
// auth.MFATrustedDevice and the operator-side
// platform.OperatorMFATrustedDevice share the formatting loop without
// either model having to satisfy a common interface.
func MapTrustedDevices[T any](rows []T, project func(T) TrustedDeviceRow) []TrustedDeviceDTO {
	out := make([]TrustedDeviceDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, NewTrustedDeviceDTO(project(r)))
	}
	return out
}

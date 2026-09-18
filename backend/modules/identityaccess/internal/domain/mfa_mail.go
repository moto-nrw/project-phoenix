package domain

import (
	"net"
	"time"
)

// AccountIdentity is the account facts the MFA gate reads: who to mail and
// whether the account is inside its failure cooldown.
type AccountIdentity struct {
	ID             int64
	Email          string
	Active         bool
	MFAAttempts    int
	MFALockedUntil *time.Time
}

// MFACodeMail is one e-mail code message. The template deliberately carries
// no link or button: the code is pasted into the moto login by hand, which
// removes the "click here to confirm" phishing pattern.
type MFACodeMail struct {
	// Recipient is the mailbox; RecipientName is empty for accounts and the
	// display name for operators.
	Recipient     string
	RecipientName string
	// ReferenceID is the account or operator the code belongs to, and
	// Operator distinguishes the two ledgers and delivery types.
	ReferenceID int64
	Operator    bool
	Code        string
	// ExpiryMinutes is the code's lifetime as the template renders it.
	ExpiryMinutes int
	// RequestIP is empty when the request carried no usable client address.
	RequestIP string
	// TrustedDeviceEnabled and TrustedDeviceDays render the
	// "remember this device" hint. Operators always have it on.
	TrustedDeviceEnabled bool
	TrustedDeviceDays    int
}

// TrustedDeviceMail notifies the account or operator holder that a
// remember-device cookie was just issued, so trusting a device never happens
// silently.
type TrustedDeviceMail struct {
	Recipient     string
	RecipientName string
	ReferenceID   int64
	Operator      bool
	// DeviceLabel is the shortened User-Agent.
	DeviceLabel string
	RequestIP   string
	// AddedAt is the German-formatted moment the device was trusted.
	AddedAt     string
	TrustedDays int
}

// MFACodeMailFor builds the code mail of one recipient.
func MFACodeMailFor(recipient, name string, referenceID int64, operator bool, code string, ttl time.Duration, ip net.IP, trustedDeviceEnabled bool, trustedDeviceDays int) MFACodeMail {
	return MFACodeMail{
		Recipient: recipient, RecipientName: name, ReferenceID: referenceID, Operator: operator,
		Code: code, ExpiryMinutes: int(ttl.Minutes()), RequestIP: AuditIPString(ip),
		TrustedDeviceEnabled: trustedDeviceEnabled, TrustedDeviceDays: trustedDeviceDays,
	}
}

// AuditIPString renders a client address for a template or an audit row and
// answers empty for a missing or unspecified one.
func AuditIPString(ip net.IP) string {
	if ip == nil || ip.IsUnspecified() {
		return ""
	}
	return ip.String()
}

// MFAAuditIP renders a client address for an audit row whose column is INET
// NOT NULL. A missing address becomes the project's sentinel so the row
// still lands instead of being rejected and lost.
func MFAAuditIP(ip net.IP) string {
	if ip == nil {
		return MFAAuditFallbackIP
	}
	return ip.String()
}

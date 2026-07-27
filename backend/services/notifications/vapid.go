package notifications

import (
	"bytes"
	"crypto/ecdh"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
)

// VAPIDConfig carries the Voluntary Application Server Identification keys
// (RFC 8292) for Web Push. A completely empty config disables Web Push; a
// partially configured or invalid config must fail service initialization.
type VAPIDConfig struct {
	PublicKey  string
	PrivateKey string
	Subscriber string // contact URI, e.g. "mailto:ops@example.org"
}

// Configured reports whether all VAPID fields are set.
func (c VAPIDConfig) Configured() bool {
	return strings.TrimSpace(c.PublicKey) != "" &&
		strings.TrimSpace(c.PrivateKey) != "" &&
		strings.TrimSpace(c.Subscriber) != ""
}

// Validate checks a configured VAPID key pair and subscriber URI. An entirely
// empty config remains valid because keyless environments deliberately disable
// Web Push.
func (c VAPIDConfig) Validate() error {
	publicKey := strings.TrimSpace(c.PublicKey)
	privateKey := strings.TrimSpace(c.PrivateKey)
	subscriber := strings.TrimSpace(c.Subscriber)
	if publicKey == "" && privateKey == "" && subscriber == "" {
		return nil
	}

	var missing []string
	if publicKey == "" {
		missing = append(missing, "VAPID_PUBLIC_KEY")
	}
	if privateKey == "" {
		missing = append(missing, "VAPID_PRIVATE_KEY")
	}
	if subscriber == "" {
		missing = append(missing, "VAPID_SUBSCRIBER")
	}
	if len(missing) > 0 {
		return fmt.Errorf("all VAPID values must be set together; missing %s", strings.Join(missing, ", "))
	}

	publicBytes, err := decodeVAPIDKey(publicKey)
	if err != nil {
		return fmt.Errorf("VAPID_PUBLIC_KEY must be URL-safe base64: %w", err)
	}
	curve := ecdh.P256()
	public, err := curve.NewPublicKey(publicBytes)
	if err != nil {
		return fmt.Errorf("VAPID_PUBLIC_KEY must be an uncompressed P-256 public key: %w", err)
	}

	privateBytes, err := decodeVAPIDKey(privateKey)
	if err != nil {
		return fmt.Errorf("VAPID_PRIVATE_KEY must be URL-safe base64: %w", err)
	}
	private, err := curve.NewPrivateKey(privateBytes)
	if err != nil {
		return fmt.Errorf("VAPID_PRIVATE_KEY must be a valid 32-byte P-256 private key: %w", err)
	}
	if !bytes.Equal(private.PublicKey().Bytes(), public.Bytes()) {
		return errors.New("VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY do not form a key pair")
	}

	if err := validateVAPIDSubscriber(subscriber); err != nil {
		return err
	}
	return nil
}

func decodeVAPIDKey(value string) ([]byte, error) {
	decoded, err := base64.URLEncoding.DecodeString(value)
	if err == nil {
		return decoded, nil
	}
	return base64.RawURLEncoding.DecodeString(value)
}

func validateVAPIDSubscriber(value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return fmt.Errorf("VAPID_SUBSCRIBER must be a valid contact URI: %w", err)
	}
	switch parsed.Scheme {
	case "mailto":
		if parsed.Opaque == "" {
			return errors.New("VAPID_SUBSCRIBER mailto URI must contain an address")
		}
		address, err := mail.ParseAddress(parsed.Opaque)
		if err != nil || address.Address != parsed.Opaque {
			return errors.New("VAPID_SUBSCRIBER mailto URI must contain a valid address")
		}
	case "https":
		if parsed.Host == "" || parsed.User != nil {
			return errors.New("VAPID_SUBSCRIBER https URI must contain a host and no credentials")
		}
	default:
		return errors.New("VAPID_SUBSCRIBER must use mailto or https")
	}
	return nil
}

// webPushSubscriber adapts the RFC contact URI to webpush-go, which adds the
// mailto scheme itself unless the value starts with https.
func (c VAPIDConfig) webPushSubscriber() string {
	return strings.TrimPrefix(c.Subscriber, "mailto:")
}

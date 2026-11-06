// Package lettermint speaks to the mail provider Lettermint: it verifies and
// reads its webhook deliveries and calls its Team API. Signature scheme and
// payloads follow https://lettermint.co/docs/platform/webhooks/ (read
// 2026-11-06).
package lettermint

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	providerName    = "lettermint"
	headerSignature = "X-Lettermint-Signature"
	headerDelivery  = "X-Lettermint-Delivery"
	signatureWindow = 5 * time.Minute
)

// The reasons a delivery is refused. The caller answers all of them alike, so
// the answer tells a sender nothing about what was wrong.
var (
	errSignatureMissing   = errors.New("lettermint webhook: signature headers are missing or malformed")
	errSignatureMismatch  = errors.New("lettermint webhook: signature does not match")
	errTimestampOutOfSpan = errors.New("lettermint webhook: timestamp is outside the accepted window")
)

// Webhook verifies and reads one provider's deliveries.
type Webhook struct {
	secrets []string
}

// NewWebhook takes the comma-separated secrets. Several secrets let a secret
// be rotated without a gap; each is used as is, with its whsec_ prefix.
func NewWebhook(secretList string) *Webhook {
	webhook := &Webhook{}
	for part := range strings.SplitSeq(secretList, ",") {
		if secret := strings.TrimSpace(part); secret != "" {
			webhook.secrets = append(webhook.secrets, secret)
		}
	}
	return webhook
}

// Name is the provider's path segment.
func (*Webhook) Name() string { return providerName }

// Configured reports whether a secret exists to verify with.
func (w *Webhook) Configured() bool { return len(w.secrets) > 0 }

// Verify checks the signature over the raw body. Lettermint signs
// "{t}.{body}" and sends t=<unix>,v1=<hex>. Its SDKs also require the delivery
// header to equal the signed time, and so does moto.
func (w *Webhook) Verify(header func(string) string, body []byte, now time.Time) error {
	stamp, provided, ok := parseSignature(header(headerSignature))
	if !ok || header(headerDelivery) != stamp {
		return errSignatureMissing
	}
	unix, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil {
		return errSignatureMissing
	}
	// The MAC runs first: the window of an unsigned time says nothing.
	if !w.matches(stamp, body, provided) {
		return errSignatureMismatch
	}
	if age := now.Sub(time.Unix(unix, 0)); age > signatureWindow || age < -signatureWindow {
		return errTimestampOutOfSpan
	}
	return nil
}

func (w *Webhook) matches(stamp string, body, provided []byte) bool {
	for _, secret := range w.secrets {
		if hmac.Equal(signatureMAC(secret, stamp, body), provided) {
			return true
		}
	}
	return false
}

func signatureMAC(secret, stamp string, body []byte) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return mac.Sum(nil)
}

func parseSignature(value string) (string, []byte, bool) {
	var stamp string
	var provided []byte
	for part := range strings.SplitSeq(value, ",") {
		key, text, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch key {
		case "t":
			stamp = text
		case "v1":
			decoded, err := hex.DecodeString(text)
			if err != nil || len(decoded) != sha256.Size {
				return "", nil, false
			}
			provided = decoded
		}
	}
	return stamp, provided, stamp != "" && provided != nil
}

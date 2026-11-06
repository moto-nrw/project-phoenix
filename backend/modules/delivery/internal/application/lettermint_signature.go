// The Lettermint files speak to that mail provider: they verify and read its
// webhook deliveries and call its Team API. Signature scheme and payloads follow
// https://lettermint.co/docs/platform/webhooks/ (read 2026-11-06).

package application

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
	lettermintProviderName    = "lettermint"
	lettermintHeaderSignature = "X-Lettermint-Signature"
	lettermintHeaderDelivery  = "X-Lettermint-Delivery"
	lettermintSignatureWindow = 5 * time.Minute
)

// The reasons a delivery is refused. The caller answers all of them alike, so
// the answer tells a sender nothing about what was wrong.
var (
	errSignatureMissing   = errors.New("lettermint webhook: signature headers are missing or malformed")
	errSignatureMismatch  = errors.New("lettermint webhook: signature does not match")
	errTimestampOutOfSpan = errors.New("lettermint webhook: timestamp is outside the accepted window")
)

// lettermintWebhook verifies and reads one provider's deliveries.
type lettermintWebhook struct {
	secrets []string
}

// newLettermintWebhook takes the comma-separated secrets. Several secrets let a secret
// be rotated without a gap; each is used as is, with its whsec_ prefix.
func newLettermintWebhook(secretList string) *lettermintWebhook {
	webhook := &lettermintWebhook{}
	for part := range strings.SplitSeq(secretList, ",") {
		if secret := strings.TrimSpace(part); secret != "" {
			webhook.secrets = append(webhook.secrets, secret)
		}
	}
	return webhook
}

// Name is the provider's path segment.
func (*lettermintWebhook) Name() string { return lettermintProviderName }

// Configured reports whether a secret exists to verify with.
func (w *lettermintWebhook) Configured() bool { return len(w.secrets) > 0 }

// Verify checks the signature over the raw body. Lettermint signs
// "{t}.{body}" and sends t=<unix>,v1=<hex>. Its SDKs also require the delivery
// header to equal the signed time, and so does moto.
func (w *lettermintWebhook) Verify(header func(string) string, body []byte, now time.Time) error {
	stamp, provided, ok := parseLettermintSignature(header(lettermintHeaderSignature))
	if !ok || header(lettermintHeaderDelivery) != stamp {
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
	if age := now.Sub(time.Unix(unix, 0)); age > lettermintSignatureWindow || age < -lettermintSignatureWindow {
		return errTimestampOutOfSpan
	}
	return nil
}

// matches accepts any provided value under any secret: during a rotation a
// sender may sign with both.
func (w *lettermintWebhook) matches(stamp string, body []byte, provided [][]byte) bool {
	for _, secret := range w.secrets {
		mac := lettermintMAC(secret, stamp, body)
		for _, value := range provided {
			if hmac.Equal(mac, value) {
				return true
			}
		}
	}
	return false
}

func lettermintMAC(secret, stamp string, body []byte) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return mac.Sum(nil)
}

// parseLettermintSignature skips a value it cannot read: another value of the
// same header may still match.
func parseLettermintSignature(value string) (string, [][]byte, bool) {
	var stamp string
	var provided [][]byte
	for part := range strings.SplitSeq(value, ",") {
		key, text, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch key {
		case "t":
			stamp = text
		case "v1":
			if decoded, err := hex.DecodeString(text); err == nil && len(decoded) == sha256.Size {
				provided = append(provided, decoded)
			}
		}
	}
	return stamp, provided, stamp != "" && provided != nil
}

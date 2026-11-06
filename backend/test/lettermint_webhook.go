package test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// SignLettermintWebhook signs a body the way Lettermint documents it, written down
// a second time so a test catches a change on moto's side. It returns the values
// of the signature and the delivery header.
func SignLettermintWebhook(secret string, at time.Time, body string) (signature, delivery string) {
	delivery = fmt.Sprint(at.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(delivery + "." + body))
	return "t=" + delivery + ",v1=" + hex.EncodeToString(mac.Sum(nil)), delivery
}

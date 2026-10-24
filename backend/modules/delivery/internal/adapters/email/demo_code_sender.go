package email

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"time"

	mailcontract "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
)

// Delivery-only protocol to the sidecar inbox in the shared loopback namespace.
const demoCodeAddress = "127.0.0.1:10026"

type demoCode struct {
	Recipient string
	Code      string
}

func sendDemoCode(ctx context.Context, address string, message mailcontract.Message) error {
	content, ok := message.Content.(map[string]any)
	if !ok {
		return mailcontract.ErrNotDeliveredInDemo
	}
	code, ok := content["Code"].(string)
	if !ok || len(code) != 6 || message.To.Address == "" {
		return mailcontract.ErrNotDeliveredInDemo
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return mailcontract.ErrNotDeliveredInDemo
	}
	defer func() { _ = conn.Close() }()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return mailcontract.ErrNotDeliveredInDemo
	}
	if json.NewEncoder(conn).Encode(demoCode{Recipient: message.To.Address, Code: code}) != nil {
		return mailcontract.ErrNotDeliveredInDemo
	}
	var ack [1]byte
	if _, err := io.ReadFull(conn, ack[:]); err != nil || ack[0] != 1 {
		return mailcontract.ErrNotDeliveredInDemo
	}
	return nil
}

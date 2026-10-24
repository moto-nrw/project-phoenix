package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// Server and sidecar share a network namespace. Only the demo transport sends
// here; the listener offers delivery, never an API for reading codes.
const demoCodeAddress = "127.0.0.1:10026"

var errDemoCodeUnavailable = errors.New("internal demo MFA code unavailable")

type demoCode struct {
	Recipient string
	Code      string
	Received  time.Time
}

// DemoCodeInbox is a process-local mailbox for the demo runner. Codes are never
// logged or written to disk. They still have to pass the ordinary MFA verifier.
type DemoCodeInbox struct {
	listener net.Listener
	done     chan struct{}
	mu       sync.Mutex
	codes    map[string]demoCode
}

// NewDemoCodeInbox starts the internal code transport only in the demo sidecar.
func NewDemoCodeInbox(appEnv string) (*DemoCodeInbox, error) {
	if !strings.EqualFold(strings.TrimSpace(appEnv), "demo") {
		return nil, errors.New("internal MFA inbox requires the demo environment")
	}
	listener, err := net.Listen("tcp", demoCodeAddress)
	if err != nil {
		return nil, err
	}
	return startDemoCodeInbox(listener), nil
}

func startDemoCodeInbox(listener net.Listener) *DemoCodeInbox {
	inbox := &DemoCodeInbox{listener: listener, done: make(chan struct{}), codes: make(map[string]demoCode)}
	go inbox.serve()
	return inbox
}

func (b *DemoCodeInbox) Close() error {
	err := b.listener.Close()
	<-b.done
	b.mu.Lock()
	clear(b.codes)
	b.mu.Unlock()
	return err
}

func (b *DemoCodeInbox) FetchCode(ctx context.Context, recipient string, since time.Time) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.prune()
	message, ok := b.codes[recipient]
	if !ok || message.Received.Before(since) {
		return "", errDemoCodeUnavailable
	}
	delete(b.codes, recipient)
	return message.Code, nil
}

func (b *DemoCodeInbox) prune() {
	cutoff := time.Now().Add(-10 * time.Minute)
	for recipient, message := range b.codes {
		if message.Received.Before(cutoff) {
			delete(b.codes, recipient)
		}
	}
}

func (b *DemoCodeInbox) serve() {
	defer close(b.done)
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			return
		}
		b.receive(conn)
		_ = conn.Close()
	}
}

func (b *DemoCodeInbox) receive(conn net.Conn) {
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return
	}
	var message demoCode
	if json.NewDecoder(io.LimitReader(conn, 4096)).Decode(&message) != nil || message.Recipient == "" || len(message.Code) != 6 {
		return
	}
	message.Received = time.Now()
	b.mu.Lock()
	b.prune()
	if len(b.codes) >= 1024 {
		b.mu.Unlock()
		return
	}
	b.codes[message.Recipient] = message
	b.mu.Unlock()
	_, _ = conn.Write([]byte{1})
}

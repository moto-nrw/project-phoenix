// Package email provides email sending functionality.
package email

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/k3a/html2text"
	"github.com/vanng822/go-premailer/premailer"
)

var templates *template.Template

type Mailer interface {
	Send(Message) error
}

// MessageIDReporter is the optional extension for API transports whose
// delivery callbacks identify a message only by a provider-side ID.
//
// PrepareProviderMessageID must reserve a stable ID without submitting the
// message. The outbox worker commits that ID before calling SendWithID, so an
// immediate callback can always correlate. SendWithID must submit with exactly
// the prepared ID.
type MessageIDReporter interface {
	PrepareProviderMessageID(Message) (providerMessageID string, err error)
	SendWithID(message Message, providerMessageID string) error
}

// Message struct holds all parts of a specific email Message.
type Message struct {
	From     Email
	To       Email
	Subject  string
	Template string
	Content  any

	// MessageID is the RFC 5322 Message-ID (without angle brackets) to stamp
	// on the outgoing mail. Empty means "let the transport decide". The outbox
	// worker always sets it, so later provider delivery events can be
	// correlated back to the outbox row.
	MessageID string

	html string
	text string
}

// parse parses the corrsponding template and content
func (m *Message) parse() error {
	buf := new(bytes.Buffer)
	if err := templates.ExecuteTemplate(buf, m.Template, m.Content); err != nil {
		return err
	}
	prem, err := premailer.NewPremailerFromString(buf.String(), premailer.NewOptions())
	if err != nil {
		return err
	}

	html, err := prem.Transform()
	if err != nil {
		return err
	}
	m.html = html

	m.text = html2text.HTML2Text(html)
	return nil
}

// Email struct holds email address and recipient name.
type Email struct {
	Name    string
	Address string
}

// NewEmail returns an email address.
func NewEmail(name, address string) Email {
	return Email{
		Name:    name,
		Address: address,
	}
}

func parseTemplates() error {
	templates = template.New("").Funcs(fMap)
	return filepath.Walk("./templates", func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, ".html") {
			_, err = templates.ParseFiles(path)
			return err
		}
		return err
	})
}

var fMap = template.FuncMap{
	"formatAsDate":     formatAsDate,
	"formatAsDuration": formatAsDuration,
}

func formatAsDate(t time.Time) string {
	year, month, day := t.Date()
	return fmt.Sprintf("%d.%d.%d", day, month, year)
}

func formatAsDuration(t time.Time) string {
	dur := time.Until(t)
	hours := int(dur.Hours())
	mins := int(dur.Minutes())

	v := ""
	if hours != 0 {
		v += strconv.Itoa(hours) + " hours and "
	}
	v += strconv.Itoa(mins) + " minutes"
	return v
}

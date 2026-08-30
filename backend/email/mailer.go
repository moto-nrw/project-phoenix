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
	"sync"
	"time"

	"github.com/k3a/html2text"
	"github.com/vanng822/go-premailer/premailer"
)

// templates is the process-wide parsed template set. NewMailer rebuilds it,
// and every Message.parse reads it, so both sides take templatesMu: without
// it two concurrent NewMailer calls (each service factory builds a mailer)
// race on the pointer and on the set being parsed into it.
var (
	templates   *template.Template
	templatesMu sync.RWMutex
)

type Mailer interface {
	Send(Message) error
}

// Message struct holds all parts of a specific email Message.
type Message struct {
	From Email
	To   Email
	// ReplyTo is where answers go when that is not the sender. Tenant-bound
	// mail (Eltern-Einladung, Anmeldung, Elternmitteilung) keeps the central
	// authenticated From and points replies at the OGS instead, so a parent
	// answering does not reach moto (#1936). The zero value emits no header,
	// which is what every global system mail keeps.
	ReplyTo  Email
	Subject  string
	Template string
	Content  any
	html     string
	text     string
}

// parse parses the corrsponding template and content
func (m *Message) parse() error {
	buf := new(bytes.Buffer)
	templatesMu.RLock()
	set := templates
	templatesMu.RUnlock()
	if err := set.ExecuteTemplate(buf, m.Template, m.Content); err != nil {
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
	set := template.New("").Funcs(fMap)
	if err := filepath.Walk("./templates", func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, ".html") {
			_, err = set.ParseFiles(path)
			return err
		}
		return err
	}); err != nil {
		return err
	}

	// Publish only a fully parsed set: a reader must never observe a
	// half-populated template set.
	templatesMu.Lock()
	templates = set
	templatesMu.Unlock()
	return nil
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

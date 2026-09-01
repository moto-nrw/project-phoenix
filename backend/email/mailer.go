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
func (m *Message) parse(templates *template.Template) error {
	if templates == nil {
		return fmt.Errorf("email templates are not configured")
	}
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

func parseTemplates(dir string) (*template.Template, error) {
	set := template.New("").Funcs(fMap)
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, ".html") {
			_, err = set.ParseFiles(path)
			return err
		}
		return err
	}); err != nil {
		return nil, err
	}
	return set, nil
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

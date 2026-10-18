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

	mailcontract "github.com/moto-nrw/project-phoenix/modules/delivery/mail"

	"github.com/k3a/html2text"
	"github.com/vanng822/go-premailer/premailer"
)

// parse parses the corrsponding template and content
type renderedMessage struct {
	mailcontract.Message
	html string
	text string
}

func (m *renderedMessage) parse(templates *template.Template) error {
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

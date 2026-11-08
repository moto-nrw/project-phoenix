// Package templates is the contract of the document templates of a Träger
// (#4677, spec #4611 user stories 91 and 92, ADR 0152): the kinds of
// document, the placeholders each kind knows, the standard template of each
// kind, the check that refuses unknown placeholders and the filling of a
// template with the values of one document. It is pure: the owner of the
// templates stores them, the owner of a document fills one, and the letter
// renderer lays the result out.
//
// A template has a subject, a salutation, a body and a closing. A
// placeholder is written {{name}}. In the body, an empty line starts a new
// paragraph, a paragraph whose first line starts with "# " has that line as
// its heading, a paragraph wrapped in ** is printed bold, and a line that
// holds nothing but a table placeholder prints the table under the
// paragraph. A paragraph whose placeholders are all empty is left out, so a
// letter without free text or emergency numbers drops their paragraph.
package templates

import (
	"errors"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Kind is the kind of document a template writes.
type Kind string

// The kinds of document of a Träger (user story 91).
const (
	KindContract     Kind = "vertrag"
	KindInvoice      Kind = "rechnung"
	KindFeeNotice    Kind = "bescheid"
	KindConfirmation Kind = "bestaetigung"
	KindCancellation Kind = "kuendigung"
	KindReminder     Kind = "mahnung"
	KindCertificate  Kind = "bescheinigung"
)

// Limits of the parts of a template, in characters.
const (
	MaxSubjectLength    = 200
	MaxSalutationLength = 200
	MaxBodyLength       = 10000
	MaxClosingLength    = 500
	// MaxLegalRemedyLength is the longest legal remedy of a fee notice
	// (#4688); the Hardt notice needs about 600 characters.
	MaxLegalRemedyLength = 5000
)

// Template is the text of a document with its placeholders.
type Template struct {
	Subject    string `json:"subject"`
	Salutation string `json:"salutation"`
	Body       string `json:"body"`
	Closing    string `json:"closing"`
	// Invoice is how the table of amounts lists the positions (#5691). Only
	// kinds that print {{positionen}} have one; the zero value is the
	// standard table.
	Invoice InvoiceLayout `json:"invoice"`
	// LegalRemedy is the Rechtsbehelfsbelehrung of a fee notice (#4688):
	// the Träger writes it, moto has no legal text of its own. Only the kind
	// fee notice has one. A fee notice prints it after the body, so no text
	// of the template can leave it out, and no fee notice is issued while it
	// is empty (LacksLegalRemedy).
	LegalRemedy string `json:"legal_remedy"`
}

// Placeholder is a name a template of a kind may use. Table placeholders
// print a two-column table when they stand alone on a line.
type Placeholder struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Table bool   `json:"table"`
}

// Row is a line of a table: a label and its value. Bold marks a total.
type Row struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Bold  bool   `json:"bold"`
}

// Value is what a placeholder stands for in one document. A table
// placeholder carries Rows; used inside a sentence it prints them as text.
type Value struct {
	Text string
	Rows []Row
}

// Block is a paragraph of the filled body.
type Block struct {
	Heading string `json:"heading"`
	Text    string `json:"text"`
	Bold    bool   `json:"bold"`
	Rows    []Row  `json:"rows"`
}

// Filled is a template filled with the values of one document. LegalRemedy
// holds the paragraphs of the legal remedy of a fee notice, empty for every
// other kind.
type Filled struct {
	Subject     string   `json:"subject"`
	Salutation  string   `json:"salutation"`
	Blocks      []Block  `json:"blocks"`
	Closing     []string `json:"closing"`
	LegalRemedy []Block  `json:"legal_remedy,omitempty"`
}

var (
	// ErrUnknownKind reports a kind of document that does not exist.
	ErrUnknownKind = errors.New("unknown kind of document")
	// ErrInvalid reports a template with a missing or too long part.
	ErrInvalid = errors.New("the template is incomplete or too long")
)

// UnknownPlaceholdersError names the placeholders a template uses that its
// kind does not know, and every placeholder written wrongly (a missing or
// extra brace) as it stands in the text.
type UnknownPlaceholdersError struct {
	Names []string
}

func (e *UnknownPlaceholdersError) Error() string {
	return "unknown placeholders: " + strings.Join(e.Names, ", ")
}

// InvalidError names the parts of a template that are missing or too long.
type InvalidError struct {
	Fields map[string]string
}

func (e *InvalidError) Error() string { return ErrInvalid.Error() }

// Unwrap lets errors.Is find ErrInvalid.
func (e *InvalidError) Unwrap() error { return ErrInvalid }

var (
	placeholderPattern = regexp.MustCompile(`\{\{\s*([^{}]*?)\s*\}\}`)
	// strayBrace finds what is left of a placeholder written wrongly, such
	// as {{kinder} or {familienname}}, after the complete ones are removed.
	strayBrace = regexp.MustCompile(`\S*[{}]\S*`)
)

// Kinds lists every kind of document in the order the Träger-Büro sees them.
func Kinds() []Kind {
	return []Kind{KindConfirmation, KindInvoice, KindFeeNotice, KindContract, KindCancellation, KindReminder, KindCertificate}
}

// Known reports whether kind is a kind of document.
func Known(kind Kind) bool {
	return slices.Contains(Kinds(), kind)
}

// Placeholders lists the placeholders a template of kind may use.
func Placeholders(kind Kind) []Placeholder {
	return slices.Clone(catalog[kind].placeholders)
}

// Default is the standard template of kind, used while the Träger has
// none of its own.
func Default(kind Kind) Template {
	return catalog[kind].standard
}

// Normalize trims every part and unifies line breaks.
func Normalize(template Template) Template {
	clean := func(text string) string {
		return strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	}
	return Template{
		Subject:     strings.Join(strings.Fields(clean(template.Subject)), " "),
		Salutation:  strings.Join(strings.Fields(clean(template.Salutation)), " "),
		Body:        clean(template.Body),
		Closing:     clean(template.Closing),
		Invoice:     template.Invoice,
		LegalRemedy: clean(template.LegalRemedy),
	}
}

// HasLegalRemedy reports whether a template of kind carries a legal remedy:
// only the fee notice, an administrative act (#4688).
func HasLegalRemedy(kind Kind) bool {
	return kind == KindFeeNotice
}

// LacksLegalRemedy reports a template of a kind that needs a legal remedy
// and has none. Saving such a template is allowed, so the Träger-Büro can
// write it in steps; issuing a fee notice with it is not (#4688): without
// the legal remedy the time limit for an appeal grows to one year (§ 58
// Abs. 2 VwGO).
func LacksLegalRemedy(kind Kind, template Template) bool {
	if !HasLegalRemedy(kind) {
		return false
	}
	// Legal text must survive even when optional placeholder values are
	// absent. A heading or a placeholder alone is no remedy text.
	for _, block := range fillLegalRemedy(template.LegalRemedy, nil) {
		if block.Text != "" {
			return false
		}
	}
	return true
}

// Check accepts a normalized template of kind: subject and body are
// required, every part keeps its limit, and every placeholder is one the
// kind knows. It reports an *InvalidError or an *UnknownPlaceholdersError.
func Check(kind Kind, template Template) error {
	if !Known(kind) {
		return ErrUnknownKind
	}
	fields := partErrors(template)
	if template.Invoice != (InvoiceLayout{}) && !HasInvoice(kind) {
		fields["invoice"] = "has no table of amounts"
	}
	if strings.TrimSpace(template.LegalRemedy) != "" && !HasLegalRemedy(kind) {
		fields["legal_remedy"] = "has no legal remedy"
	}
	if len(fields) > 0 {
		return &InvalidError{Fields: fields}
	}
	if names := unknownPlaceholders(kind, template); len(names) > 0 {
		return &UnknownPlaceholdersError{Names: names}
	}
	return nil
}

// partErrors names the parts that are missing or too long.
func partErrors(template Template) map[string]string {
	fields := map[string]string{}
	for _, part := range []struct {
		name, text string
		max        int
		required   bool
	}{
		{"subject", template.Subject, MaxSubjectLength, true},
		{"salutation", template.Salutation, MaxSalutationLength, false},
		{"body", template.Body, MaxBodyLength, true},
		{"closing", template.Closing, MaxClosingLength, false},
		{"legal_remedy", template.LegalRemedy, MaxLegalRemedyLength, false},
	} {
		switch {
		case part.required && strings.TrimSpace(part.text) == "":
			fields[part.name] = "is required"
		case len([]rune(part.text)) > part.max:
			fields[part.name] = "is too long"
		}
	}
	return fields
}

// unknownPlaceholders lists, sorted and once each, the placeholders of the
// template its kind does not know and every placeholder written wrongly.
func unknownPlaceholders(kind Kind, template Template) []string {
	unknown := map[string]bool{}
	for _, text := range []string{template.Subject, template.Salutation, template.Body, template.Closing, template.LegalRemedy} {
		for _, match := range placeholderPattern.FindAllStringSubmatch(text, -1) {
			if match[1] == "" || !knows(kind, match[1]) {
				unknown[cmpOr(match[1], match[0])] = true
			}
		}
		// A brace outside a complete placeholder is a placeholder written
		// wrongly; the letter would print it as it stands.
		for _, stray := range strayBrace.FindAllString(placeholderPattern.ReplaceAllString(text, " "), -1) {
			unknown[stray] = true
		}
	}
	names := make([]string, 0, len(unknown))
	for name := range unknown {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func cmpOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// Fill fills a template with the values of one document. A placeholder
// without a value prints nothing.
func Fill(template Template, values map[string]Value) Filled {
	subject, _ := fillLine(template.Subject, values)
	salutation, _ := fillLine(template.Salutation, values)
	closing, _ := fillLine(template.Closing, values)
	return Filled{
		Subject:     oneLine(subject),
		Salutation:  oneLine(salutation),
		Blocks:      fillBody(template.Body, values, true),
		Closing:     lines(closing),
		LegalRemedy: fillLegalRemedy(template.LegalRemedy, values),
	}
}

// fillLegalRemedy fills the legal remedy; a template without one has none.
func fillLegalRemedy(text string, values map[string]Value) []Block {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return fillBody(text, values, false)
}

func knows(kind Kind, name string) bool {
	for _, placeholder := range catalog[kind].placeholders {
		if placeholder.Name == name {
			return true
		}
	}
	return false
}

// fillLine replaces the placeholders of a text. empty reports a text that
// uses placeholders, all of which print nothing.
func fillLine(text string, values map[string]Value) (string, bool) {
	used, filled := 0, 0
	result := placeholderPattern.ReplaceAllStringFunc(text, func(match string) string {
		used++
		value := values[placeholderPattern.FindStringSubmatch(match)[1]]
		inline := value.inline()
		if inline != "" {
			filled++
		}
		return inline
	})
	return result, used > 0 && filled == 0
}

func (v Value) inline() string {
	if v.Text != "" || len(v.Rows) == 0 {
		return v.Text
	}
	parts := make([]string, len(v.Rows))
	for i, row := range v.Rows {
		parts[i] = strings.TrimSpace(row.Label + " " + row.Value)
	}
	return strings.Join(parts, "; ")
}

func fillBody(body string, values map[string]Value, omitEmpty bool) []Block {
	blocks := []Block{}
	for _, paragraph := range paragraphs(body) {
		blocks = append(blocks, fillParagraph(paragraph, values, omitEmpty)...)
	}
	return blocks
}

// paragraphs splits a body at its empty lines.
func paragraphs(body string) [][]string {
	var result [][]string
	var current []string
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			if len(current) > 0 {
				result = append(result, current)
			}
			current = nil
			continue
		}
		current = append(current, strings.TrimRight(line, " \t"))
	}
	if len(current) > 0 {
		result = append(result, current)
	}
	return result
}

// fillParagraph keeps standalone tables in order with the surrounding text.
// When omitEmpty is true, a paragraph whose placeholders are all empty is
// left out. Legal-remedy paragraphs keep their literal text instead.
func fillParagraph(paragraph []string, values map[string]Value, omitEmpty bool) []Block {
	var blocks []Block
	var block Block
	var count placeholderCount
	if heading, ok := strings.CutPrefix(paragraph[0], "# "); ok {
		block.Heading = strings.TrimSpace(count.fill(heading, values))
		paragraph = paragraph[1:]
	}
	appendBlock := func(textLines []string, rows []Row) {
		text, bold := boldText(strings.TrimSpace(strings.Join(textLines, "\n")))
		block.Text = strings.TrimSpace(count.fill(text, values))
		block.Bold = bold && block.Text != ""
		block.Rows = rows
		if block.Heading != "" || block.Text != "" || len(block.Rows) > 0 {
			blocks = append(blocks, block)
		}
		block = Block{}
	}
	start := 0
	for i, line := range paragraph {
		if rows, ok := tableLine(line, values); ok {
			count.add(len(rows) > 0)
			appendBlock(paragraph[start:i], rows)
			start = i + 1
		}
	}
	appendBlock(paragraph[start:], nil)
	if omitEmpty && count.allEmpty() {
		return nil
	}
	return blocks
}

// placeholderCount counts the placeholders of a paragraph and those that
// print something.
type placeholderCount struct{ used, filled int }

func (c *placeholderCount) add(filled bool) {
	c.used++
	if filled {
		c.filled++
	}
}

func (c *placeholderCount) fill(text string, values map[string]Value) string {
	result, empty := fillLine(text, values)
	if placeholderPattern.MatchString(text) {
		c.add(!empty)
	}
	return result
}

func (c placeholderCount) allEmpty() bool { return c.used > 0 && c.filled == 0 }

// boldText strips the ** around a whole paragraph and reports it.
func boldText(text string) (string, bool) {
	inner, ok := strings.CutPrefix(text, "**")
	if !ok {
		return text, false
	}
	if inner, ok = strings.CutSuffix(inner, "**"); !ok {
		return text, false
	}
	return strings.TrimSpace(inner), true
}

// tableLine reports a line that holds nothing but a table placeholder.
func tableLine(line string, values map[string]Value) ([]Row, bool) {
	match := placeholderPattern.FindStringSubmatch(strings.TrimSpace(line))
	if match == nil || match[0] != strings.TrimSpace(line) || !tableNames[match[1]] {
		return nil, false
	}
	rows := values[match[1]].Rows
	if rows == nil {
		rows = []Row{}
	}
	return rows, true
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func lines(text string) []string {
	result := []string{}
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result
}

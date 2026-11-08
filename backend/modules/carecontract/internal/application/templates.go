package application

import (
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// acceptanceText is the complete, stable text of the template conditions.
// The parent submission must have checksummed and accepted this exact text,
// not a generic reference to terms. Explicit labels and indented values keep
// multiline notes from masquerading as another condition. A missing partner
// cannot bind the eventual fallback from the mutable letterhead.
func acceptanceText(terms carecontract.TemplateInput) string {
	if strings.TrimSpace(terms.ContractPartner) == "" {
		return ""
	}
	values := [][2]string{
		{"Laufzeit", TermText(terms, nil)},
		{"Kündigung", NoticeText(terms)},
		{"Vertragspartner", terms.ContractPartner},
		{"Sonderkündigung", "Nein"},
		{"Satzung", "Nein"},
	}
	if terms.SpecialTermination {
		values[3][1] = "Ja\n" + terms.SpecialTerminationNote
	}
	if terms.StatuteRequired {
		values[4][1] = "Ja\n" + terms.StatuteTitle
		values = append(values, [2]string{"Link zur Satzung", terms.StatuteURL})
	}
	lines := []string{"Vertragsbedingungen für die Betreuung"}
	for _, value := range values {
		lines = append(lines, value[0]+":\n  "+strings.ReplaceAll(value[1], "\n", "\n  "))
	}
	return strings.TrimSpace(strings.Join(lines, "\n\n"))
}

// The limits of a template, as the table checks them.
const (
	maxTermMonths       = 48
	maxNoticeMonths     = 12
	maxSpecialNote      = 1000
	maxStatuteTitle     = 200
	maxStatuteURL       = 500
	maxContractPartner  = 300
	fieldRequired       = "required"
	fieldTooLong        = "too_long"
	fieldOutOfRange     = "out_of_range"
	fieldUnknown        = "unknown"
	fieldInvalidAddress = "invalid_url"
)

// normalizeTemplate trims the texts and drops what an unchosen option
// carries, so a switched-off special termination keeps no stale note.
func normalizeTemplate(input carecontract.TemplateInput) carecontract.TemplateInput {
	input.SpecialTerminationNote = strings.TrimSpace(input.SpecialTerminationNote)
	input.StatuteTitle = strings.TrimSpace(input.StatuteTitle)
	input.StatuteURL = strings.TrimSpace(input.StatuteURL)
	input.ContractPartner = strings.TrimSpace(input.ContractPartner)
	if input.TermKind != carecontract.TermFixedMonths {
		input.TermMonths = nil
	}
	if !input.SpecialTermination {
		input.SpecialTerminationNote = ""
	}
	if !input.StatuteRequired {
		input.StatuteTitle, input.StatuteURL = "", ""
	}
	return input
}

func validateTemplate(input carecontract.TemplateInput) map[string]string {
	fields := map[string]string{}
	if !slices.Contains([]string{carecontract.TermOpenEnded, carecontract.TermSchoolYear, carecontract.TermFixedMonths}, input.TermKind) {
		fields["term_kind"] = fieldUnknown
	}
	if input.TermKind == carecontract.TermFixedMonths && (input.TermMonths == nil || *input.TermMonths < 1 || *input.TermMonths > maxTermMonths) {
		fields["term_months"] = fieldOutOfRange
	}
	if input.NoticeMonths < 0 || input.NoticeMonths > maxNoticeMonths {
		fields["notice_months"] = fieldOutOfRange
	}
	if !slices.Contains([]string{carecontract.NoticeMonthEnd, carecontract.NoticeHalfYear, carecontract.NoticeSchoolYearEnd}, input.NoticeTo) {
		fields["notice_to"] = fieldUnknown
	}
	textField(fields, "special_termination_note", input.SpecialTerminationNote, input.SpecialTermination, maxSpecialNote)
	textField(fields, "statute_title", input.StatuteTitle, input.StatuteRequired, maxStatuteTitle)
	textField(fields, "contract_partner", input.ContractPartner, false, maxContractPartner)
	if input.StatuteURL != "" {
		parsed, err := url.Parse(input.StatuteURL)
		switch {
		case utf8.RuneCountInString(input.StatuteURL) > maxStatuteURL:
			fields["statute_url"] = fieldTooLong
		case err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "":
			fields["statute_url"] = fieldInvalidAddress
		}
	}
	return fields
}

func textField(fields map[string]string, name, value string, required bool, limit int) {
	switch {
	case required && value == "":
		fields[name] = fieldRequired
	case utf8.RuneCountInString(value) > limit:
		fields[name] = fieldTooLong
	}
}

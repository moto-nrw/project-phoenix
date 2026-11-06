package textnorm

import "testing"

func TestContentDispositionKeepsTheAccentedName(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		disposition, filename, want string
	}{
		{"attachment", "gruppe-ćirić.pdf", `attachment; filename="gruppe-ciric.pdf"; filename*=UTF-8''gruppe-%C4%87iri%C4%87.pdf`},
		{"attachment", "anmeldungen-müller.xlsx", `attachment; filename="anmeldungen-mueller.xlsx"; filename*=UTF-8''anmeldungen-m%C3%BCller.xlsx`},
		{"inline", "Attest José.pdf", `inline; filename="Attest Jose.pdf"; filename*=UTF-8''Attest%20Jos%C3%A9.pdf`},
		{"attachment", "liste-学校.csv", `attachment; filename="liste-__.csv"; filename*=UTF-8''liste-%E5%AD%A6%E6%A0%A1.csv`},
		{"attachment", "export.pdf", `attachment; filename="export.pdf"`},
		{"attachment", "Dienstplan 2026-07-27.pdf", `attachment; filename="Dienstplan 2026-07-27.pdf"`},
		{"attachment", `a"b\c;d%e.pdf`, `attachment; filename="a_b_c;d_e.pdf"; filename*=UTF-8''a%22b%5Cc%3Bd%25e.pdf`},
		{"attachment", "zeile\r\nX-Evil: 1.csv", `attachment; filename="zeile__X-Evil: 1.csv"; filename*=UTF-8''zeile%0D%0AX-Evil%3A%201.csv`},
		{"attachment", "", "attachment"},
	} {
		if got := ContentDisposition(tc.disposition, tc.filename); got != tc.want {
			t.Errorf("ContentDisposition(%q, %q)\n got %s\nwant %s", tc.disposition, tc.filename, got, tc.want)
		}
	}
}

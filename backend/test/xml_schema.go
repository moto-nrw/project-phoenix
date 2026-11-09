package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// AssertXMLMatchesSchema checks an XML document against an XSD with
// xmllint (libxml2), which the development machines and the CI runners
// carry. The SEPA direct debit file (#4697) validates against the ISO
// schema this way; a test without xmllint fails rather than skipping.
func AssertXMLMatchesSchema(tb testing.TB, document []byte, schema string) {
	tb.Helper()
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		tb.Fatalf("xmllint is required to check XML against its schema: %v", err)
	}
	path := filepath.Join(tb.TempDir(), "document.xml")
	if err := os.WriteFile(path, document, 0o600); err != nil {
		tb.Fatalf("write the XML document: %v", err)
	}
	out, err := exec.Command(xmllint, "--noout", "--schema", schema, path).CombinedOutput() //nolint:gosec // test paths only
	if err != nil {
		tb.Fatalf("the document does not validate against %s: %v\n%s", filepath.Base(schema), err, out)
	}
}

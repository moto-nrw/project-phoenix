package carecontracthttp

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// The paper copy keeps the name it was uploaded under; a quote in it must
// not end the filename parameter and smuggle in a second one.
func TestWriteFile_EscapesHostileFilename(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteFile(rec, carecontract.File{
		Filename: `x"; filename*=UTF-8''x.html; a=".pdf`, ContentType: "application/pdf", Data: []byte("%PDF-1.4"),
	}, "attachment")

	assert.Equal(t, `attachment; filename="x_; filename*=UTF-8''x.html; a=_.pdf"; filename*=UTF-8''x%22%3B%20filename%2A%3DUTF-8%27%27x.html%3B%20a%3D%22.pdf`,
		rec.Header().Get("Content-Disposition"))
}

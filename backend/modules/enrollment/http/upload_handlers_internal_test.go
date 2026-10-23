package enrollmenthttp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtensionMatchesContent(t *testing.T) {
	t.Parallel()

	assert.True(t, extensionMatchesContent("Bescheid.PDF", "application/pdf"))
	assert.True(t, extensionMatchesContent("foto.jpeg", "image/jpeg"))
	assert.True(t, extensionMatchesContent("IMG_0001.jpg", "image/jpeg"))
	assert.True(t, extensionMatchesContent("scan.png", "image/png"))
	assert.False(t, extensionMatchesContent("bescheid.png", "application/pdf"), "a renamed PDF is refused")
	assert.False(t, extensionMatchesContent("bescheid", "application/pdf"), "a file without extension is refused")
	assert.False(t, extensionMatchesContent("brief.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"))
}

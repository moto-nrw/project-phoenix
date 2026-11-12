package application

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
)

// The one rule for the name of a Träger-Büro person (#4693): trimmed, 1 to
// 100 characters, no control character, valid UTF-8.
func TestNormalizeTraegerMemberName(t *testing.T) {
	t.Parallel()
	for _, valid := range []struct{ in, want string }{
		{"Jana Krause", "Jana Krause"},
		{"  Jana Krause  ", "Jana Krause"},
		{"Dr. Ömer Çelik-Weiß", "Dr. Ömer Çelik-Weiß"},
		{strings.Repeat("ä", identityaccess.TraegerMemberNameMaxLength), strings.Repeat("ä", identityaccess.TraegerMemberNameMaxLength)},
	} {
		got, err := normalizeTraegerMemberName(valid.in)
		require.NoError(t, err, valid.in)
		assert.Equal(t, valid.want, got)
	}
	for _, invalid := range []string{"", "   ", "Jana\tKrause", "Jana\nKrause", "Jana\u0000", "Jana\u0085Krause",
		strings.Repeat("ä", identityaccess.TraegerMemberNameMaxLength+1), "Jana\xffKrause"} {
		_, err := normalizeTraegerMemberName(invalid)
		assert.ErrorIs(t, err, identityaccess.ErrTraegerInvalidName, "%q", invalid)
	}
}

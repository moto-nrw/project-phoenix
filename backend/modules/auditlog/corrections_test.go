package auditlog

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCorrectionActorUsesFrozenNameThenEmail(t *testing.T) {
	t.Parallel()
	name, blank, email := "  Review Actor  ", " ", " actor@example.test "
	require.Equal(t, "Review Actor", (DirectCorrection{ActorNameSnapshot: &name, ActorEmailSnapshot: &email}).ActorName())
	require.Equal(t, "actor@example.test", (DirectCorrection{ActorNameSnapshot: &blank, ActorEmailSnapshot: &email}).ActorName())
	require.Equal(t, "Unbekannt", (DirectCorrection{}).ActorName())
}

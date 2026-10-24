package peopledirectory_test

import (
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/stretchr/testify/assert"
)

func TestPhotoUnlinker_RequiresUploadStorage(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(t, "student photo unlinker: upload storage is required", func() {
		peopledirectory.NewPhotoUnlinker(slog.Default(), "public", nil)
	})
}

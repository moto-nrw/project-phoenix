package architecture

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/architecture/artifacts"
)

func readMaterializedPolicy(t *testing.T, path string) string {
	t.Helper()
	data, err := artifacts.ReadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func readMaterializedEffects(t *testing.T, path string) string {
	t.Helper()
	stored, err := artifacts.ReadInventory(path, artifacts.CommandEffects, false)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := decodeCommandEffects(stored.Document)
	if err != nil {
		t.Fatal(err)
	}
	data, err := marshalCommandEffects(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

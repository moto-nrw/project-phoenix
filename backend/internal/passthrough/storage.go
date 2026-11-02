package passthrough

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/moto-nrw/project-phoenix/architecture/artifacts"
)

// ImportSnapshot stores an explicitly prepared review document as owner-local
// intent/evidence. It checks current source but does not grant an ADR or bypass
// Check's independent immutable-base comparison.
func ImportSnapshot(root, filename string) error {
	candidate, err := readRegistry(filename)
	if err != nil {
		return err
	}
	measured, err := Scan(root)
	if err != nil {
		return err
	}
	problems := CompareSites(candidate.Sites, measured)
	if candidate.Scope != scope() {
		problems = append(problems, "snapshot scope does not match scanner selectors")
	}
	for _, site := range candidate.Sites {
		if site.Reason == "" || site.Decision == "" {
			problems = append(problems, "missing reason/decision: "+site.ID)
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("snapshot does not match reviewed source:\n%s", strings.Join(problems, "\n"))
	}
	document, err := json.Marshal(candidate)
	if err != nil {
		return err
	}
	return artifacts.WriteInventory(filepath.Join(root, "architecture/passthrough.json"), artifacts.Passthrough, document)
}

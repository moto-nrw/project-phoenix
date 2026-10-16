package test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var productionSourceIndexes sync.Map

// productionGoSources uses the compiler's default build context, even when
// this test binary was built with testhooks. Share one listing per source
// tree across parallel source ratchets.
func productionGoSources(root string) (map[string]bool, error) {
	load, _ := productionSourceIndexes.LoadOrStore(root, sync.OnceValues(func() (map[string]bool, error) {
		command := exec.Command("go", "list", "-tags=", "-f", `{{range .GoFiles}}{{$.Dir}}/{{.}}{{"\n"}}{{end}}{{range .CgoFiles}}{{$.Dir}}/{{.}}{{"\n"}}{{end}}`, "./...")
		command.Dir = root
		output, err := command.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("list production source files: %w: %s", err, strings.TrimSpace(string(output)))
		}
		sources := make(map[string]bool)
		for _, path := range strings.Split(string(output), "\n") {
			if path != "" {
				sources[filepath.Clean(path)] = true
			}
		}
		return sources, nil
	}))
	return load.(func() (map[string]bool, error))()
}

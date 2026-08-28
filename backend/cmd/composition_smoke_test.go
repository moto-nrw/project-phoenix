package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func assertCommandComposition(t *testing.T) {
	t.Helper()
	want := compositionCommandPaths(t)
	got := registeredCommandPaths(RootCmd)
	if !slices.Equal(got, want) {
		t.Fatalf("registered command paths changed\nwant: %v\ngot:  %v", want, got)
	}

	for _, path := range want {
		arguments := strings.Fields(strings.TrimPrefix(path, RootCmd.Name()))
		command, remaining, err := RootCmd.Find(arguments)
		if err != nil {
			t.Errorf("parse %q: %v", path, err)
			continue
		}
		if command.CommandPath() != path || len(remaining) != 0 {
			t.Errorf("parse %q resolved command %q with remaining args %v", path, command.CommandPath(), remaining)
		}
	}
}

func compositionCommandPaths(t *testing.T) []string {
	t.Helper()
	file, err := os.Open(filepath.Join("..", "architecture", "composition.json")) // #nosec G304 -- fixed repository manifest
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	var inventory struct {
		Commands []string `json:"commands"`
	}
	if err := json.NewDecoder(file).Decode(&inventory); err != nil {
		t.Fatal(err)
	}
	return inventory.Commands
}

func registeredCommandPaths(root *cobra.Command) []string {
	var paths []string
	var walk func(*cobra.Command)
	walk = func(command *cobra.Command) {
		paths = append(paths, command.CommandPath())
		for _, child := range command.Commands() {
			walk(child)
		}
	}
	walk(root)
	sort.Strings(paths)
	return paths
}

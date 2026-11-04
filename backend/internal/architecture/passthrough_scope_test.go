package architecture

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/gitfixture"
	"github.com/moto-nrw/project-phoenix/internal/passthrough"
)

// Exercise the reported bypass with an actually rebuilt scanner, not a forged
// scope string: skip api/, regenerate the inventory, then compare to its base.
func TestPassthroughRejectsAPIFilterBypass(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	root := filepath.Join(repo, "backend")
	put := func(path, source string) {
		t.Helper()
		path = filepath.Join(repo, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	put("backend/api/port.go", "package api\nfunc (s *Port) Find() error { return s.store.Find() }")
	put("docs/adr/0090-method-evidence.md", "# Evidence\n")
	var output bytes.Buffer
	if err := passthrough.WriteSnapshot(&output, root, "0090"); err != nil {
		t.Fatal(err)
	}
	put("backend/architecture/passthrough.json", output.String())
	gitfixture.Init(t, repo)
	for _, args := range [][]string{
		{"add", "."},
		{"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "Base"},
	} {
		runGit(t, repo, args...)
	}
	tool := filepath.Join(repo, "tool")
	put("tool/go.mod", "module github.com/moto-nrw/project-phoenix\n\ngo 1.24\n")
	files, err := filepath.Glob(filepath.Join(packageDir(t), "..", "passthrough", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		file = filepath.Base(file)
		if file == "scanner.go" {
			source = strings.Replace(source, "if entry.IsDir() {", "if entry.IsDir() {\nif entry.Name() == \"api\" { return filepath.SkipDir }", 1)
		}
		put("tool/internal/passthrough/"+file, source)
	}
	storageFiles, err := filepath.Glob(filepath.Join(packageDir(t), "..", "..", "architecture", "artifacts", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range storageFiles {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		put("tool/architecture/artifacts/"+filepath.Base(file), string(data))
	}
	put("tool/main.go", `package main
import ("os"; "fmt"; "github.com/moto-nrw/project-phoenix/internal/passthrough")
func main() {
 root := os.Args[1]
 f, err := os.Create(root+"/architecture/passthrough.json")
 if err != nil { panic(err) }
 if err := passthrough.WriteSnapshot(f, root, os.Args[2]); err != nil { panic(err) }
 if err := f.Close(); err != nil { panic(err) }
 if err := passthrough.Check(root, "HEAD"); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
}`)
	data, err := processOutput(tool, gitTestEnvironment(), "go", "run", ".", root, "0090")
	if err == nil {
		t.Fatalf("api/ filter bypass accepted after snapshot regeneration: %s", data)
	}
	if !strings.Contains(string(data), "scan selectors changed without a new ADR") || !strings.Contains(string(data), "disappeared method still matches: api/Port.Find") {
		t.Fatalf("want scope and source-removal refusals, got: %s", data)
	}
	t.Logf("api/ bypass rejected after snapshot regeneration: %s", data)
	put("docs/adr/0091-new-scan.md", "# New scope\n")
	data, err = processOutput(tool, gitTestEnvironment(), "go", "run", ".", root, "0091")
	if err == nil || !strings.Contains(string(data), "disappeared method still matches: api/Port.Find") || strings.Contains(string(data), "scan selectors changed") {
		t.Fatalf("new scope ADR must not excuse a still-present method: %s", data)
	}
}

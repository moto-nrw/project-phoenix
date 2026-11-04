package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/gitfixture"
)

func TestFixtureIgnoresInheritedGitState(t *testing.T) {
	t.Parallel()
	if os.Getenv("PHX_GIT_FIXTURE_CHILD") == "1" {
		root := t.TempDir()
		gitfixture.Init(t, root)
		gitfixture.Run(t, root, "commit", "--allow-empty", "-qm", "isolated fixture")
		if got := strings.TrimSpace(gitfixture.Run(t, root, "log", "-1", "--format=%an <%ae>")); got != "Git fixture <fixture@example.invalid>" {
			t.Fatalf("inherited author: %s", got)
		}
		if config := gitfixture.Run(t, root, "config", "--list"); strings.Contains(config, "fixture.inherited") {
			t.Fatal("fixture inherited caller configuration")
		}
		if hooks := strings.TrimSpace(gitfixture.Run(t, root, "config", "--local", "--get", "core.hooksPath")); hooks != os.DevNull {
			t.Fatalf("fixture hooks enabled: %s", hooks)
		}
		return
	}

	root := t.TempDir()
	hooks := filepath.Join(root, "hooks")
	if err := os.MkdirAll(hooks, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\nprintf ran > \"$PHX_GIT_FIXTURE_HOOK_MARKER\"\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	config := "[core]\n\thooksPath = " + filepath.ToSlash(hooks) + "\n[commit]\n\tgpgsign = true\n[gpg]\n\tprogram = missing-fixture-gpg\n[fixture]\n\tinherited = true\n"
	global := filepath.Join(root, "global.gitconfig")
	if err := os.WriteFile(global, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "hook-ran")
	environment := append(gitfixture.Environment(os.Environ()),
		"PHX_GIT_FIXTURE_CHILD=1", "PHX_GIT_FIXTURE_HOOK_MARKER="+marker,
		"GIT_CONFIG_GLOBAL="+global, "GIT_CONFIG_SYSTEM="+global, "GIT_CONFIG_NOSYSTEM=0",
		"GIT_CONFIG="+global, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0="+hooks,
		"GIT_CONFIG_PARAMETERS='fixture.inherited=parameters'",
		"GIT_DIR="+filepath.Join(root, "foreign.git"), "GIT_WORK_TREE="+root,
		"GIT_AUTHOR_NAME=Inherited author", "GIT_AUTHOR_EMAIL=inherited@example.invalid",
		"GIT_TEMPLATE_DIR="+root,
	)
	if output, err := processOutput(packageDir(t), environment, os.Args[0], "-test.run=^TestFixtureIgnoresInheritedGitState$", "-test.v"); err != nil {
		t.Fatalf("fixture failed with hostile Git state: %v\n%s", err, output)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("inherited hook ran: %v", err)
	}
	if after, err := os.ReadFile(global); err != nil || string(after) != config {
		t.Fatalf("caller configuration changed: %v", err)
	}
}

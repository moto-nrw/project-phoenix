package artifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const effectFixture = `{"schema_version":1,"entries":[{"source_owner":"alpha","port":"alpha.Read","binding":"alpha.Owner.Read","target":"alpha.Store.Read","binding_digest":"frozen","effect":"query","classification":"local","reason":"Reviewed query"}]}`
const methodFixture = `{"scope":"reviewed-scope","decision":"0090","sites":[{"id":"alpha/Owner.Read","owner":"source/alpha","build":"production","form":"forward-debt","evidence":"exact body","reason":"reviewed exception","decision":"0090"}]}`

func TestReviewedInventoryRoundTripPreservesEveryField(t *testing.T) {
	t.Parallel()
	for _, example := range []struct {
		kind   InventoryKind
		source string
	}{{CommandEffects, effectFixture}, {Passthrough, methodFixture}} {
		t.Run(string(example.kind), func(t *testing.T) {
			t.Parallel()
			file := filepath.Join(t.TempDir(), string(example.kind)+".json")
			if err := WriteInventory(file, example.kind, []byte(example.source)); err != nil {
				t.Fatal(err)
			}
			stored, err := ReadInventory(file, example.kind, true)
			if err != nil {
				t.Fatal(err)
			}
			assertSameJSON(t, []byte(example.source), stored.Document)
			if !stored.Fragmented {
				t.Fatal("inventory did not use owner fragments")
			}
			// Original materialized bases remain readable, without a new directory.
			legacy := filepath.Join(t.TempDir(), string(example.kind)+".json")
			if err := os.WriteFile(legacy, []byte(example.source), 0600); err != nil {
				t.Fatal(err)
			}
			old, err := ReadInventory(legacy, example.kind, true)
			if err != nil || old.Fragmented {
				t.Fatalf("legacy base: %v", err)
			}
			assertSameJSON(t, old.Document, stored.Document)
		})
	}
}

func TestInventoryRejectsMissingCounterpartAndEvidenceInReview(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	file := filepath.Join(root, "command-effects.json")
	if err := WriteInventory(file, CommandEffects, []byte(effectFixture)); err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(root, "command-effects", "evidence", "alpha.json")
	if err := os.Remove(evidence); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadInventory(file, CommandEffects, true); err == nil || !strings.Contains(err.Error(), "counterpart") {
		t.Fatalf("missing evidence accepted: %v", err)
	}
	if err := WriteInventory(file, CommandEffects, []byte(effectFixture)); err != nil {
		t.Fatal(err)
	}
	review := filepath.Join(root, "command-effects", "reviews", "alpha.json")
	data, err := os.ReadFile(review)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), `"effect": "query"`, `"binding_digest": "forged", "effect": "query"`, 1))
	if err := os.WriteFile(review, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadInventory(file, CommandEffects, false); err == nil || !strings.Contains(err.Error(), "may not contain binding_digest") {
		t.Fatalf("review changed evidence: %v", err)
	}
}

func TestEmptyInventoryIsExplicitAndNeedsNoTrackedEmptyDirectories(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	file := filepath.Join(root, "command-effects.json")
	if err := WriteInventory(file, CommandEffects, []byte(`{"schema_version":1,"entries":[]}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "command-effects")); err != nil {
		t.Fatal(err)
	}
	stored, err := ReadInventory(file, CommandEffects, true)
	if err != nil {
		t.Fatal(err)
	}
	assertSameJSON(t, []byte(`{"schema_version":1,"entries":[]}`), stored.Document)
}

func assertSameJSON(t *testing.T, want, got []byte) {
	t.Helper()
	var expected, actual any
	if err := json.Unmarshal(want, &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &actual); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("materialized document changed:\nwant %s\ngot %s", want, got)
	}
}

func TestInventoryPublicationPreservesUnrelatedFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	filename := filepath.Join(root, "command-effects.json")
	if err := WriteInventory(filename, CommandEffects, []byte(effectFixture)); err != nil {
		t.Fatal(err)
	}
	note := filepath.Join(root, "command-effects", "review-notes.md")
	if err := os.WriteFile(note, []byte("Keep this note"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteInventory(filename, CommandEffects, []byte(effectFixture)); err == nil {
		t.Fatal("unrelated file was silently replaced")
	}
	if data, err := os.ReadFile(note); err != nil || string(data) != "Keep this note" {
		t.Fatalf("unrelated file changed: %s, %v", data, err)
	}
}

func TestFailedInventoryHeaderPublicationRestoresPreviousFragments(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	filename := filepath.Join(root, "command-effects.json")
	if err := WriteInventory(filename, CommandEffects, []byte(effectFixture)); err != nil {
		t.Fatal(err)
	}
	review := filepath.Join(root, "command-effects", "reviews", "alpha.json")
	want, err := os.ReadFile(review)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filename, filename+".previous"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filename, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filename, "protected"), []byte("Keep"), 0600); err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(effectFixture, "Reviewed query", "Changed reason", 1)
	if err := WriteInventory(filename, CommandEffects, []byte(changed)); err == nil {
		t.Fatal("header publication unexpectedly succeeded")
	}
	got, err := os.ReadFile(review)
	if err != nil || string(got) != string(want) {
		t.Fatalf("previous reviewed source lost: %s, %v", got, err)
	}
	if data, err := os.ReadFile(filepath.Join(filename, "protected")); err != nil || string(data) != "Keep" {
		t.Fatalf("protected directory changed: %s, %v", data, err)
	}
}

func TestInventoryHeaderCannotHideOrphanFragments(t *testing.T) {
	t.Parallel()
	for _, replacement := range []string{"missing", `{"schema_version":1,"entries":[]}`} {
		t.Run(replacement, func(t *testing.T) {
			t.Parallel()
			file := filepath.Join(t.TempDir(), "command-effects.json")
			if err := WriteInventory(file, CommandEffects, []byte(effectFixture)); err != nil {
				t.Fatal(err)
			}
			if replacement == "missing" {
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(file, []byte(replacement), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadInventory(file, CommandEffects, false); err == nil || !strings.Contains(err.Error(), "orphan") || os.IsNotExist(err) {
				t.Fatalf("orphan records can be treated as absent: %v", err)
			}
		})
	}
}

func TestInventoryRejectsDirectorySymlinksBeforeAcceptingCandidate(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"command-effects", "command-effects/reviews", "command-effects/evidence"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			file := filepath.Join(root, "command-effects.json")
			if err := WriteInventory(file, CommandEffects, []byte(effectFixture)); err != nil {
				t.Fatal(err)
			}
			original := filepath.Join(root, filepath.FromSlash(boundary))
			moved := original + "-source"
			if err := os.Rename(original, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Base(moved), original); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadInventory(file, CommandEffects, false); err == nil {
				t.Fatal("candidate accepts directory symlink that immutable Git cannot read")
			}
		})
	}
}

package artifacts

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Inventory keeps the original representation distinction only for legacy
// byte-canonicality checks; semantic consumers always use the complete Document.
type Inventory struct {
	Document   []byte
	Fragmented bool
}
type InventoryKind string

const (
	CommandEffects InventoryKind = "command-effects"
	Passthrough    InventoryKind = "passthrough"
)
const inventoryStorage = "reviewed-parts-v1"

type inventorySpec struct {
	collection, owner          string
	identity, evidence, review []string
}

func inventoryFormat(kind InventoryKind) (inventorySpec, error) {
	switch kind {
	case CommandEffects:
		return inventorySpec{"entries", "source_owner", []string{"source_owner", "port", "binding", "target", "target_owner"}, []string{"binding_digest"}, []string{"effect", "classification", "adr", "approval_adr", "reason"}}, nil
	case Passthrough:
		return inventorySpec{"sites", "owner", []string{"id"}, []string{"owner", "build", "form", "evidence"}, []string{"reason", "decision"}}, nil
	default:
		return inventorySpec{}, fmt.Errorf("unknown inventory kind %q", kind)
	}
}

func ReadInventory(filename string, kind InventoryKind, canonical bool) (Inventory, error) {
	return ReadInventoryFS(os.DirFS(filepath.Dir(filename)), filepath.Base(filename), kind, canonical)
}

func ReadInventoryFS(source fs.FS, filename string, kind InventoryKind, canonical bool) (Inventory, error) {
	data, err := fs.ReadFile(source, filename)
	if err != nil {
		if orphan := rejectOrphanParts(source, filename, string(kind)); orphan != nil {
			return Inventory{}, orphan
		}
		return Inventory{}, err
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(data, &header); err != nil {
		return Inventory{}, err
	}
	marker, fragmented := header["storage"]
	if !fragmented {
		if err := rejectOrphanParts(source, filename, string(kind)); err != nil {
			return Inventory{}, err
		}
		return Inventory{Document: data}, nil
	}
	return readFragmentedInventory(source, filename, kind, canonical, header, data, marker)
}

func readFragmentedInventory(source fs.FS, filename string, kind InventoryKind, canonical bool, header map[string]json.RawMessage, data, marker []byte) (Inventory, error) {
	var format string
	if err := json.Unmarshal(marker, &format); err != nil || format != inventoryStorage {
		return Inventory{}, fmt.Errorf("unsupported inventory storage %s", marker)
	}
	spec, err := inventoryFormat(kind)
	if err != nil {
		return Inventory{}, err
	}
	if _, mixed := header[spec.collection]; mixed {
		return Inventory{}, fmt.Errorf("mixed inventory storage")
	}
	if canonical {
		if err := checkCanonical(data); err != nil {
			return Inventory{}, fmt.Errorf("%s: %w", filename, err)
		}
	}
	delete(header, "storage")
	empty := string(header["empty"]) == "true"
	if _, present := header["empty"]; present && !empty {
		return Inventory{}, fmt.Errorf("invalid empty inventory marker")
	}
	delete(header, "empty")
	root := path.Join(path.Dir(filename), string(kind))
	entries, err := joinInventoryParts(source, root, spec, canonical, empty)
	if err != nil {
		return Inventory{}, err
	}
	header[spec.collection], err = json.Marshal(entries)
	if err != nil {
		return Inventory{}, err
	}
	document, err := json.Marshal(header)
	return Inventory{Document: document, Fragmented: true}, err
}

type inventoryRecord struct {
	owner  string
	fields map[string]json.RawMessage
}

func readInventoryParts(source fs.FS, directory string, spec inventorySpec, side string, canonical bool, empty bool) (map[string]inventoryRecord, error) {
	files, err := readArtifactDirectory(source, directory)
	if empty && os.IsNotExist(err) {
		return map[string]inventoryRecord{}, nil
	}
	if empty && len(files) > 0 {
		return nil, fmt.Errorf("empty inventory contains fragments")
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", directory, err)
	}
	records := map[string]inventoryRecord{}
	for _, file := range files {
		owner, part, err := readInventoryPart(source, directory, file, spec, canonical)
		if err != nil {
			return nil, err
		}
		if err := addInventoryRecords(records, owner, part, spec, side, canonical); err != nil {
			return nil, err
		}
	}

	return records, nil
}

func joinInventoryParts(source fs.FS, root string, spec inventorySpec, canonical bool, empty bool) ([]map[string]json.RawMessage, error) {
	reviews, err := readInventoryParts(source, path.Join(root, "reviews"), spec, "reviews", canonical, empty)
	if err != nil {
		return nil, err
	}
	evidence, err := readInventoryParts(source, path.Join(root, "evidence"), spec, "evidence", canonical, empty)
	if err != nil {
		return nil, err
	}
	if !empty && len(reviews) == 0 && len(evidence) == 0 {
		return nil, fmt.Errorf("nonempty inventory has no fragments")
	}
	if len(reviews) != len(evidence) {
		return nil, fmt.Errorf("inventory has missing review or evidence counterparts")
	}
	keys := make([]string, 0, len(reviews))
	for key := range reviews {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	records := make([]map[string]json.RawMessage, 0, len(keys))
	for _, key := range keys {
		record, err := joinInventoryRecord(key, reviews[key], evidence, spec)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func validateInventoryFields(fields map[string]json.RawMessage, spec inventorySpec, side string) error {
	allowed := map[string]bool{}
	for _, field := range spec.identity {
		allowed[field] = true
	}
	additions := spec.review
	if side == "" {
		additions = append(append([]string{}, spec.review...), spec.evidence...)
	}
	if side == "evidence" {
		additions = spec.evidence
	}
	for _, field := range additions {
		allowed[field] = true
	}
	for field := range fields {
		if !allowed[field] {
			return fmt.Errorf("%s fragment may not contain %s", side, field)
		}
	}
	return nil
}
func inventoryKey(fields map[string]json.RawMessage, identity []string) (string, error) {
	values := make([]string, 0, len(identity))
	for _, field := range identity {
		var value string
		raw, exists := fields[field]
		if field == "target_owner" && !exists {
			values = append(values, "")
			continue
		}
		if err := json.Unmarshal(raw, &value); err != nil || (value == "" && field != "target_owner") {
			return "", fmt.Errorf("missing inventory identity %s", field)
		}
		values = append(values, value)
	}
	encoded, err := json.Marshal(values)
	return string(encoded), err
}
func checkCanonical(data []byte) error {
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return err
	}
	expected, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	if string(data) != string(append(expected, '\n')) {
		return fmt.Errorf("inventory fragment must be canonical JSON")
	}
	return nil
}

func readInventoryPart(source fs.FS, directory string, file fs.DirEntry, spec inventorySpec, canonical bool) (string, []map[string]json.RawMessage, error) {
	if file.IsDir() || file.Type()&fs.ModeSymlink != 0 || !strings.HasSuffix(file.Name(), ".json") {
		return "", nil, fmt.Errorf("invalid inventory fragment %s", file.Name())
	}
	owner, err := url.PathUnescape(strings.TrimSuffix(file.Name(), ".json"))
	if err != nil || owner == "" || url.PathEscape(owner)+".json" != file.Name() {
		return "", nil, fmt.Errorf("invalid inventory owner filename %s", file.Name())
	}
	data, err := fs.ReadFile(source, path.Join(directory, file.Name()))
	if err != nil {
		return "", nil, err
	}
	if canonical {
		if err := checkCanonical(data); err != nil {
			return "", nil, fmt.Errorf("%s: %w", file.Name(), err)
		}
	}
	var part map[string][]map[string]json.RawMessage
	if err := json.Unmarshal(data, &part); err != nil {
		return "", nil, err
	}
	if len(part) != 1 || len(part[spec.collection]) == 0 {
		return "", nil, fmt.Errorf("empty or unknown inventory fragment %s", file.Name())
	}
	return owner, part[spec.collection], nil
}

func addInventoryRecords(records map[string]inventoryRecord, owner string, part []map[string]json.RawMessage, spec inventorySpec, side string, canonical bool) error {
	previous := ""
	for _, fields := range part {
		if err := validateInventoryFields(fields, spec, side); err != nil {
			return err
		}
		key, err := inventoryKey(fields, spec.identity)
		if err != nil {
			return err
		}
		if canonical && previous > key {
			return fmt.Errorf("inventory fragment identities must be sorted")
		}
		previous = key
		if _, exists := records[key]; exists {
			return fmt.Errorf("duplicate %s inventory identity %s", side, key)
		}
		records[key] = inventoryRecord{owner: owner, fields: fields}
	}
	return nil
}

func joinInventoryRecord(key string, review inventoryRecord, evidence map[string]inventoryRecord, spec inventorySpec) (map[string]json.RawMessage, error) {
	observed, ok := evidence[key]
	if !ok {
		return nil, fmt.Errorf("missing evidence counterpart for %s", key)
	}
	if review.owner != observed.owner {
		return nil, fmt.Errorf("review/evidence owner differs for %s", key)
	}
	for field, value := range observed.fields {
		review.fields[field] = value
	}
	var owner string
	if err := json.Unmarshal(review.fields[spec.owner], &owner); err != nil || owner != review.owner {
		return nil, fmt.Errorf("misplaced inventory owner for %s", key)
	}
	return review.fields, nil
}

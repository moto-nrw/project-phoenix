package artifacts

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

type policyPart struct {
	owner    string
	sections map[string][]json.RawMessage
}

func readPolicyParts(source fs.FS, directory string) ([]policyPart, error) {
	files, err := readArtifactDirectory(source, directory)
	if err != nil {
		return nil, fmt.Errorf("policy fragments: %w", err)
	}
	var parts []policyPart
	for _, file := range files {
		part, err := readPolicyPart(source, directory, file)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("no policy fragments in %s", directory)
	}
	return parts, nil
}

func assemblePolicy(header map[string]json.RawMessage, parts []policyPart) ([]byte, error) {
	owners := map[string]string{}
	for _, part := range parts {
		for _, raw := range part.sections["packages"] {
			var pkg struct{ Path, Owner string }
			if err := json.Unmarshal(raw, &pkg); err != nil {
				return nil, err
			}
			owners[pkg.Path] = pkg.Owner
		}
	}
	for section, identity := range policySections {
		records, err := assembleSection(parts, section, identity, owners)
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(records)
		if err != nil {
			return nil, err
		}
		header[section] = data
	}
	return json.Marshal(header)
}

func assembleSection(parts []policyPart, section, identity string, owners map[string]string) ([]json.RawMessage, error) {
	byID := map[string]json.RawMessage{}
	for _, part := range parts {
		for _, raw := range part.sections[section] {
			if err := addPolicyRecord(byID, raw, part.owner, section, identity, owners); err != nil {
				return nil, err
			}
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]json.RawMessage, 0, len(ids))
	for _, id := range ids {
		result = append(result, byID[id])
	}
	return result, nil
}

func validatePolicyOwner(owner, section string, record map[string]json.RawMessage, packages map[string]string) error {
	field := map[string]string{"packages": "owner", "data_objects": "write_owner", "rules": "source_owner", "read_projections": "package"}[section]
	var expected string
	if raw, ok := record[field]; ok {
		if err := json.Unmarshal(raw, &expected); err != nil {
			return err
		}
	}
	if section == "read_projections" {
		expected = packages[expected]
	}
	if expected == "" {
		expected = "global"
	}
	if owner != expected {
		return fmt.Errorf("%s fragment cannot own %s record for %s", owner, section, expected)
	}
	return nil
}

func readPolicyPart(source fs.FS, directory string, file fs.DirEntry) (policyPart, error) {
	if file.IsDir() || !ownerFile.MatchString(file.Name()) || file.Type()&fs.ModeSymlink != 0 {
		return policyPart{}, fmt.Errorf("invalid policy fragment %s", file.Name())
	}
	data, err := fs.ReadFile(source, path.Join(directory, file.Name()))
	if err != nil {
		return policyPart{}, err
	}
	part := policyPart{owner: strings.TrimSuffix(file.Name(), ".json")}
	if err := json.Unmarshal(data, &part.sections); err != nil {
		return policyPart{}, err
	}
	if len(part.sections) == 0 {
		return policyPart{}, fmt.Errorf("empty policy fragment %s", file.Name())
	}
	for section, records := range part.sections {
		if _, ok := policySections[section]; !ok {
			return policyPart{}, fmt.Errorf("unknown policy fragment section %s", section)
		}
		if len(records) == 0 {
			return policyPart{}, fmt.Errorf("empty policy section %s in %s", section, file.Name())
		}
	}
	return part, nil
}

func addPolicyRecord(byID map[string]json.RawMessage, raw json.RawMessage, owner, section, identity string, packages map[string]string) error {
	var record map[string]json.RawMessage
	if err := json.Unmarshal(raw, &record); err != nil {
		return err
	}
	var id string
	if err := json.Unmarshal(record[identity], &id); err != nil || id == "" {
		return fmt.Errorf("missing %s identity", section)
	}
	if _, exists := byID[id]; exists {
		return fmt.Errorf("duplicate %s identity %s", section, id)
	}
	if err := validatePolicyOwner(owner, section, record, packages); err != nil {
		return err
	}
	byID[id] = raw
	return nil
}

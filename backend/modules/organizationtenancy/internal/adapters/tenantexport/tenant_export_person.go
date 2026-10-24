package tenantexport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// Selection operates on the ONE catalog executor's safe, school-scoped rows.
// No additional SQL, credential reads, or unscoped identity lookups occur here.
// Only sparse keys/edges stay in memory; row contents are streamed twice.
type personKey struct{ table, column, value string }
type personNode struct {
	table    string
	keys     []personKey
	links    []personKey
	own      []personKey
	selected bool
	related  bool
}
type personSelection struct {
	rows              map[string][]*personNode
	byKey             map[personKey][]*personNode
	dependents        map[personKey][]*personNode
	unassignedObjects int64
}

func parseExportPerson(value string) (string, string, error) {
	kind, id, ok := strings.Cut(value, ":")
	table := map[string]string{"kind": "users.student_profiles", "guardian": "users.guardian_profiles", "staff": "users.staff_school_memberships"}[kind]
	n, err := strconv.ParseInt(id, 10, 64)
	if !ok || table == "" || err != nil || n <= 0 || strconv.FormatInt(n, 10) != id {
		return "", "", errors.New("tenant export: person must be kind:<profile ID>, guardian:<profile ID> or staff:<school membership ID>")
	}
	return table, id, nil
}

func (r *exportRun) selectPerson(ctx context.Context) error {
	p := &personSelection{rows: make(map[string][]*personNode), byKey: make(map[personKey][]*personNode), dependents: make(map[personKey][]*personNode)}
	if err := p.index(ctx, r); err != nil {
		return err
	}
	table, id, err := parseExportPerson(r.options.Person)
	if err != nil {
		return err
	}
	roots := p.byKey[personKey{table, "id", id}]
	if len(roots) != 1 {
		return errors.New("tenant export: person unavailable in authorized school; historical-only subjects require manual review")
	}
	p.choose(roots[0])
	r.person = p
	r.manifest.ReviewRequired = true
	// Discard full-school hashes and object references. Rebuild both from only
	// the selected records before copying any original bytes.
	r.manifest.Artifacts = nil
	r.references = make(map[string]bool)
	for i := range r.manifest.Tables {
		entry := &r.manifest.Tables[i]
		if entry.Classification == "include" {
			entry.Rows, err = r.filterPersonTable(ctx, entry.Name)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *personSelection) index(ctx context.Context, r *exportRun) error {
	columns := personIndexColumns(r.models, r.catalog)
	for _, model := range r.models {
		relations := personRelationships(model, r.catalog)
		err := scanPersonRows(ctx, filepath.Join(r.options.Output, model.Name+".jsonl"), func(fields map[string]json.RawMessage) error {
			node := newPersonNode(model.Name, fields, columns[model.Name], relations)
			p.rows[model.Name] = append(p.rows[model.Name], node)
			for _, key := range node.keys {
				p.byKey[key] = append(p.byKey[key], node)
			}
			for _, key := range node.links {
				p.dependents[key] = append(p.dependents[key], node)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func personIndexColumns(models []organizationtenancy.ExportModelTable, catalog organizationtenancy.DataLifecycleCatalog) map[string]map[string]bool {
	// Index every referenced target column, not just id (e.g. person_id).
	columns := map[string]map[string]bool{}
	for _, model := range models {
		for _, rel := range personRelationships(model, catalog) {
			if columns[rel.TargetTable] == nil {
				columns[rel.TargetTable] = map[string]bool{}
			}
			columns[rel.TargetTable][rel.TargetColumn] = true
		}
	}
	return columns
}

func newPersonNode(table string, fields map[string]json.RawMessage, columns map[string]bool, relations []organizationtenancy.ExportRelationship) *personNode {
	node := &personNode{table: table}
	if personIdentity(table) {
		node.own = appendPersonKey(node.own, "users.persons", "id", fields["person_id"])
		node.own = appendPersonKey(node.own, "auth.accounts", "id", fields["account_id"])
	}
	node.keys = appendPersonKey(node.keys, table, "id", fields["id"])
	for column := range columns {
		if column != "id" {
			node.keys = appendPersonKey(node.keys, table, column, fields[column])
		}
	}
	for _, rel := range relations {
		node.links = appendPersonKey(node.links, rel.TargetTable, rel.TargetColumn, fields[rel.Column])
	}
	node.links = append(node.links, personMetadataLinks(table, fields)...)
	return node
}

func appendPersonKey(keys []personKey, table, column string, raw json.RawMessage) []personKey {
	var text string
	if len(raw) > 0 && raw[0] == '"' && json.Unmarshal(raw, &text) == nil && text != "" {
		return append(keys, personKey{table, column, "string:" + text})
	}
	// PostgreSQL integer IDs must never pass through float64.
	n, err := strconv.ParseInt(string(raw), 10, 64)
	if err == nil && n > 0 {
		return append(keys, personKey{table, column, strconv.FormatInt(n, 10)})
	}
	return keys
}

func personIdentity(table string) bool {
	switch table {
	case "auth.accounts", "users.persons", "users.student_profiles", "users.guardian_profiles", "users.staff_school_memberships":
		return true
	}
	return false
}

func (p *personSelection) choose(root *personNode) {
	// Only the subject's own person/account is an identity seed. A guardian or
	// actor mentioned in a child's record is context, never a new subject.
	queue := []*personNode{root}
	root.selected = true
	for i := 0; i < len(queue); i++ {
		for _, own := range p.targets(queue[i].own) {
			if !own.selected {
				own.selected = true
				queue = append(queue, own)
			}
		}
		queue[i].selected, queue[i].related = true, true
	}
	// Reverse edges find data ABOUT these seeds and their dependent records.
	// Identity tables cannot be selected because somebody authored a record.
	for i := 0; i < len(queue); i++ {
		for _, node := range p.children(queue[i]) {
			if !node.selected && !personIdentity(node.table) {
				node.selected, node.related = true, true
				queue = append(queue, node)
			}
		}
	}
	p.addContext(queue)
}

func (p *personSelection) addContext(queue []*personNode) {
	for i := 0; i < len(queue); i++ {
		node := queue[i]
		// Do not traverse other people's identity chains.
		if !node.related && personIdentity(node.table) {
			continue
		}
		for _, parent := range p.targets(node.links) {
			if !parent.selected {
				parent.selected = true
				queue = append(queue, parent)
			}
		}
		for _, child := range p.children(node) {
			if !child.selected && p.contentChild(node, child) {
				child.selected = true
				queue = append(queue, child)
			}
		}
	}
}

func (p *personSelection) contentChild(parent, child *personNode) bool {
	if !personContentChild(parent.table, child.table) {
		return false
	}
	if parent.table == "enrollment.requests" && child.table == "enrollment.change_requests" {
		// A shared request can contain several siblings. Child-specific changes
		// are context only when that child was independently selected.
		for _, key := range child.links {
			if key.table == "enrollment.request_children" {
				for _, subject := range p.byKey[key] {
					return subject.related
				}
				return false
			}
		}
	}
	return true
}

func (p *personSelection) targets(keys []personKey) []*personNode {
	var nodes []*personNode
	for _, key := range keys {
		nodes = append(nodes, p.byKey[key]...)
	}
	return nodes
}

func (p *personSelection) children(node *personNode) []*personNode {
	var nodes []*personNode
	for _, key := range node.keys {
		nodes = append(nodes, p.dependents[key]...)
	}
	return nodes
}

func scanPersonRows(ctx context.Context, path string, visit func(map[string]json.RawMessage) error) error {
	f, err := os.Open(path)
	if err != nil {
		return errors.New("tenant export: person source unavailable")
	}
	defer func() { _ = f.Close() }()
	decoder := json.NewDecoder(f)
	for {
		if err := ctx.Err(); err != nil {
			return errors.New("tenant export: person selection interrupted")
		}
		var fields map[string]json.RawMessage
		if err := decoder.Decode(&fields); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return errors.New("tenant export: invalid person source")
		}
		if err := visit(fields); err != nil {
			return err
		}
	}
}

func (r *exportRun) filterPersonTable(ctx context.Context, table string) (int64, error) {
	name := table + ".jsonl"
	path := filepath.Join(r.options.Output, name)
	f, err := os.OpenFile(path+".person", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, errors.New("tenant export: cannot create person artifact")
	}
	defer func() { _ = f.Close() }()
	var row int
	var count int64
	err = scanPersonRows(ctx, path, func(fields map[string]json.RawMessage) error {
		node := r.person.rows[table][row]
		row++
		if !node.selected {
			return nil
		}
		data, err := json.Marshal(fields)
		if err != nil {
			return exportTableError(table)
		}
		if err := r.collectReferences(table, data); err != nil {
			return err
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return errors.New("tenant export: cannot write person artifact")
		}
		count++
		return nil
	})
	if err != nil {
		return 0, err
	}
	if f.Sync() != nil || f.Close() != nil || os.Rename(path+".person", path) != nil {
		return 0, errors.New("tenant export: cannot persist person artifact")
	}
	if err := r.recordArtifact(name); err != nil {
		return 0, err
	}
	return count, nil
}

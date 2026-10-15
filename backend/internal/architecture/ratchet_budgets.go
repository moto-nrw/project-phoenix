package architecture

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The source ratchets in test/*ratchet*_test.go keep their budgets and
// allowlists as Go literals. They only fall (ADR 0057 point 5): a raise
// against the pull request's base fails here, so the test file cannot be
// edited to fit the code. Compared by declaration name across all those
// files, because the files share one package:
//
//   - every top-level integer constant (a threshold or a register total; a
//     raise loosens both);
//   - every value of a top-level map[string]int literal (a budget or an
//     allowed count), and the sum of the map, so a new key cannot add what an
//     existing one may not;
//   - the number of entries of a top-level map[string]string literal (an
//     exception register keyed by function or file, valued by its reason).
//
// Other literals (selector sets, scanned directories) are configuration whose
// direction this comparison cannot judge. A declaration that is new in the
// candidate has no base and starts the measurement; one that disappears is not
// compared, so renaming a register to escape it needs review.
//
// A raise is accepted only when the comment on the raised entry, or on its
// declaration, cites an ADR ("ADR 0123") whose file docs/adr/0123-*.md this
// change adds: a raised budget needs an ADR.
type ratchetBudgets struct {
	constants  map[string]ratchetValue
	budgets    map[string]map[string]ratchetValue
	budgetDocs map[string]string
	exceptions map[string]ratchetValue
}

type ratchetValue struct {
	value    int
	location string
	comment  string
}

var ratchetBudgetFile = regexp.MustCompile(`^test/[^/]*ratchet[^/]*_test\.go$`)

var ratchetADRCitation = regexp.MustCompile(`\bADR (\d{4})\b`)

// CompareRatchetBudgets rejects a raised budget, allowlist or threshold of the
// source ratchets against the base commit.
func CompareRatchetBudgets(project, ref string) error {
	root, err := gitOutput(project, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	root = strings.TrimSpace(root)
	sha, err := resolveBaseCommit(root, ref)
	if err != nil {
		return err
	}
	prefix, err := repositoryRelativePath(root, project, project)
	if err != nil {
		return err
	}
	prefix = filepath.ToSlash(prefix)
	listing, err := gitOutput(root, "ls-tree", "-r", "--name-only", "-z", sha, "--", prefix)
	if err != nil {
		return err
	}
	base := newRatchetBudgets()
	for _, name := range strings.Split(listing, "\x00") {
		relative := strings.TrimPrefix(strings.TrimPrefix(name, prefix), "/")
		if prefix == "." {
			relative = name
		}
		if name == "" || !ratchetBudgetFile.MatchString(relative) {
			continue
		}
		source, err := readGitBlob(root, project, sha, relative)
		if err != nil {
			return err
		}
		if err := base.add(relative, source); err != nil {
			return err
		}
	}
	candidate := newRatchetBudgets()
	paths, err := filepath.Glob(filepath.Join(project, "test", "*ratchet*_test.go"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		relative := "test/" + filepath.Base(path)
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := candidate.add(relative, source); err != nil {
			return err
		}
	}
	adrs, err := addedADRs(root, sha)
	if err != nil {
		return err
	}
	raises := compareRatchetBudgets(base, candidate, adrs)
	if len(raises) != 0 {
		return fmt.Errorf("ratchet budgets raised against base %s; a raise needs an ADR this change adds (ADR 0057 point 5):\n  %s", sha, strings.Join(raises, "\n  "))
	}
	fmt.Printf("ratchet budgets passed: %d constants, %d budget registers, %d exception registers\n", len(candidate.constants), len(candidate.budgets), len(candidate.exceptions))
	return nil
}

// addedADRs lists the numbers of the ADR files the candidate tree has and
// the base commit has not.
func addedADRs(root, sha string) (map[string]bool, error) {
	listing, err := gitOutput(root, "ls-tree", "--name-only", "-z", sha, "--", "docs/adr/")
	if err != nil {
		return nil, err
	}
	existing := map[string]bool{}
	for _, name := range strings.Split(listing, "\x00") {
		existing[filepath.Base(name)] = true
	}
	paths, err := filepath.Glob(filepath.Join(root, "docs", "adr", "[0-9][0-9][0-9][0-9]-*.md"))
	if err != nil {
		return nil, err
	}
	added := map[string]bool{}
	for _, path := range paths {
		name := filepath.Base(path)
		if !existing[name] {
			added[name[:4]] = true
		}
	}
	return added, nil
}

func newRatchetBudgets() *ratchetBudgets {
	return &ratchetBudgets{
		constants: map[string]ratchetValue{}, budgets: map[string]map[string]ratchetValue{},
		budgetDocs: map[string]string{}, exceptions: map[string]ratchetValue{},
	}
}

func (r *ratchetBudgets) add(path string, source []byte) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse ratchet source: %w", err)
	}
	comments := ast.NewCommentMap(fset, file, file.Comments)
	location := func(node ast.Node) string { return fset.Position(node.Pos()).String() }
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST && general.Tok != token.VAR {
			continue
		}
		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			doc := commentText(comments, general) + commentText(comments, value)
			for index, name := range value.Names {
				if index >= len(value.Values) {
					continue
				}
				expression := value.Values[index]
				if general.Tok == token.CONST {
					if number, ok := ratchetInteger(expression); ok {
						r.constants[name.Name] = ratchetValue{number, location(name), doc}
					}
					continue
				}
				literal, ok := expression.(*ast.CompositeLit)
				if !ok {
					continue
				}
				switch ratchetMapValue(literal.Type) {
				case "int":
					entries := map[string]ratchetValue{}
					for _, element := range literal.Elts {
						pair, ok := element.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						key, keyOK := ratchetString(pair.Key)
						number, numberOK := ratchetInteger(pair.Value)
						if keyOK && numberOK {
							entries[key] = ratchetValue{number, location(pair), commentText(comments, pair)}
						}
					}
					r.budgets[name.Name] = entries
					r.budgetDocs[name.Name] = doc
				case "string":
					r.exceptions[name.Name] = ratchetValue{len(literal.Elts), location(name), doc}
				}
			}
		}
	}
	return nil
}

func commentText(comments ast.CommentMap, node ast.Node) string {
	var text strings.Builder
	for _, group := range comments[node] {
		text.WriteString(group.Text())
	}
	return text.String()
}

func ratchetMapValue(expression ast.Expr) string {
	mapping, ok := expression.(*ast.MapType)
	if !ok {
		return ""
	}
	key, keyOK := mapping.Key.(*ast.Ident)
	value, valueOK := mapping.Value.(*ast.Ident)
	if !keyOK || !valueOK || key.Name != "string" {
		return ""
	}
	return value.Name
}

func ratchetInteger(expression ast.Expr) (int, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.INT {
		return 0, false
	}
	number, err := strconv.ParseInt(strings.ReplaceAll(literal.Value, "_", ""), 0, 64)
	return int(number), err == nil
}

func ratchetString(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

func compareRatchetBudgets(base, candidate *ratchetBudgets, adrs map[string]bool) []string {
	var raises []string
	justified := func(comments ...string) bool {
		for _, comment := range comments {
			for _, match := range ratchetADRCitation.FindAllStringSubmatch(comment, -1) {
				if adrs[match[1]] {
					return true
				}
			}
		}
		return false
	}
	for name, was := range base.constants {
		now, ok := candidate.constants[name]
		if ok && now.value > was.value && !justified(now.comment) {
			raises = append(raises, fmt.Sprintf("constant %s: %d -> %d at %s", name, was.value, now.value, now.location))
		}
	}
	for name, entries := range base.budgets {
		current, ok := candidate.budgets[name]
		if !ok {
			continue
		}
		doc := candidate.budgetDocs[name]
		before, after := 0, 0
		for key, was := range entries {
			before += was.value
			if now, ok := current[key]; ok && now.value > was.value && !justified(now.comment, doc) {
				raises = append(raises, fmt.Sprintf("%s[%q]: %d -> %d at %s", name, key, was.value, now.value, now.location))
			}
		}
		for key, now := range current {
			// An entry raised or added under an added ADR keeps its base share.
			if justified(now.comment, doc) {
				now = entries[key]
			}
			after += now.value
		}
		if after > before {
			raises = append(raises, fmt.Sprintf("%s: entries sum %d -> %d (a new or moved entry may not add to the register)", name, before, after))
		}
	}
	for name, was := range base.exceptions {
		now, ok := candidate.exceptions[name]
		if ok && now.value > was.value && !justified(now.comment) {
			raises = append(raises, fmt.Sprintf("%s: %d -> %d entries at %s", name, was.value, now.value, now.location))
		}
	}
	sort.Strings(raises)
	return raises
}

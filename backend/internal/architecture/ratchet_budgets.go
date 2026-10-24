package architecture

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The source ratchets in test/*ratchet*_test.go keep their budgets and
// allowlists in Go declarations. They only fall (ADR 0057 point 5): a raise
// against the pull request's base fails here, so the test file cannot be
// edited to fit the code. The measurements, compared by declaration name
// across all those files because they share one package:
//
//   - every top-level integer constant (a threshold or a register total; a
//     raise loosens both);
//   - every value of a top-level map[string]int register (a budget or an
//     allowed count), and the sum of the register, so a new key cannot add
//     what an existing one may not;
//   - the number of entries of a top-level map[string]string register (an
//     exception register keyed by function or file, valued by its reason).
//
// Values come from go/types: the ratchet files are type-checked together, so
// every Go constant expression counts with its compiler value (typed
// conversions and complements, iota, references to other constants), and a
// register may have a named map type. A base measurement whose candidate
// declaration has no such value is an error, never a silent drop. Other literals (selector sets, scanned
// directories) are configuration whose direction this comparison cannot judge.
//
// A declaration that is new in the candidate has no base and starts a
// measurement. A base measurement may not disappear: it stays, its register
// was empty, or a candidate declaration of the same kind says "renamed from
// <old name>" in its comment and is compared with the old base values. Two
// successors for one name, or one declaration naming two predecessors, fail. Moving
// a measurement out of the ratchet files therefore fails as well.
//
// A raise is accepted only when a comment on the raised entry or its
// declaration cites an ADR ("ADR 0123") with a number the base does not have
// and the change adds as docs/adr/0123-*.md: a raised budget needs a new
// decision, and renaming an existing ADR file is none. A measurement that
// disappears needs the same: a comment naming it that cites an ADR the change
// adds.
type ratchetBudgets struct {
	fset  *token.FileSet
	files []ratchetFile

	finalized   bool
	constants   map[string]ratchetValue
	budgets     map[string]ratchetRegister
	exceptions  map[string]ratchetValue
	unsupported map[string]string
	comments    []string
}

type ratchetFile struct {
	syntax   *ast.File
	comments ast.CommentMap
}

type ratchetValue struct {
	value    int
	location string
	comment  string
}

type ratchetRegister struct {
	entries  map[string]ratchetValue
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
	candidate.finalize()
	fmt.Printf("ratchet budgets passed: %d constants, %d budget registers, %d exception registers\n", len(candidate.constants), len(candidate.budgets), len(candidate.exceptions))
	return nil
}

// addedADRs lists the ADR numbers the candidate tree has and the base commit
// has not. A number the base already holds is no new decision, whatever its
// file is called now.
func addedADRs(root, sha string) (map[string]bool, error) {
	listing, err := gitOutput(root, "ls-tree", "--name-only", "-z", sha, "--", "docs/adr/")
	if err != nil {
		return nil, err
	}
	existing := map[string]bool{}
	for _, name := range strings.Split(listing, "\x00") {
		if number, ok := adrNumber(filepath.Base(name)); ok {
			existing[number] = true
		}
	}
	paths, err := filepath.Glob(filepath.Join(root, "docs", "adr", "*.md"))
	if err != nil {
		return nil, err
	}
	added := map[string]bool{}
	for _, path := range paths {
		if number, ok := adrNumber(filepath.Base(path)); ok && !existing[number] {
			added[number] = true
		}
	}
	return added, nil
}

var adrFileName = regexp.MustCompile(`^(\d{4})-.*\.md$`)

func adrNumber(name string) (string, bool) {
	match := adrFileName.FindStringSubmatch(name)
	if match == nil {
		return "", false
	}
	return match[1], true
}

func newRatchetBudgets() *ratchetBudgets {
	return &ratchetBudgets{fset: token.NewFileSet()}
}

func (r *ratchetBudgets) add(path string, source []byte) error {
	file, err := parser.ParseFile(r.fset, path, source, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse ratchet source: %w", err)
	}
	r.files = append(r.files, ratchetFile{syntax: file, comments: ast.NewCommentMap(r.fset, file, file.Comments)})
	r.finalized = false
	return nil
}

// finalize type-checks the added files together with go/types and reads the
// measurements from the checked declarations, so a value is whatever the Go
// compiler makes of it: typed conversions, complements, iota and references
// to other constants included. Imports resolve to empty packages; a value
// that depends on one, or on a declaration outside the ratchet files, cannot
// be determined and counts as unmeasurable.
func (r *ratchetBudgets) finalize() {
	if r.finalized {
		return
	}
	r.finalized = true
	r.constants, r.budgets = map[string]ratchetValue{}, map[string]ratchetRegister{}
	r.exceptions, r.unsupported, r.comments = map[string]ratchetValue{}, map[string]string{}, nil
	if len(r.files) == 0 {
		return
	}
	syntax := make([]*ast.File, 0, len(r.files))
	for _, file := range r.files {
		syntax = append(syntax, file.syntax)
		for _, group := range file.syntax.Comments {
			r.comments = append(r.comments, group.Text())
		}
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}}
	config := types.Config{Importer: emptyImporter{}, Error: func(error) {}}
	// Type errors leave the affected declarations without a value; they are
	// reported per measurement below.
	_, _ = config.Check(syntax[0].Name.Name, r.fset, syntax, info)
	for _, file := range r.files {
		for _, declaration := range file.syntax.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.CONST && general.Tok != token.VAR {
				continue
			}
			for _, spec := range general.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				doc := commentText(file.comments, general) + commentText(file.comments, value)
				for index, name := range value.Names {
					if name.Name == "_" {
						continue
					}
					if general.Tok == token.CONST {
						r.measureConstant(info, name, doc)
						continue
					}
					var expression ast.Expr
					if index < len(value.Values) {
						expression = value.Values[index]
					}
					r.measureRegister(info, file, name, expression, doc)
				}
			}
		}
	}
}

func (r *ratchetBudgets) measureConstant(info *types.Info, name *ast.Ident, doc string) {
	object, ok := info.Defs[name].(*types.Const)
	if !ok || object.Val().Kind() != constant.Int {
		r.unsupported[name.Name] = "no integer constant the type checker can determine"
		return
	}
	number, exact := constant.Int64Val(object.Val())
	if !exact {
		r.unsupported[name.Name] = "integer constant out of range"
		return
	}
	r.constants[name.Name] = ratchetValue{int(number), r.fset.Position(name.Pos()).String(), doc}
}

func (r *ratchetBudgets) measureRegister(info *types.Info, file ratchetFile, name *ast.Ident, expression ast.Expr, doc string) {
	object, ok := info.Defs[name].(*types.Var)
	if !ok {
		r.unsupported[name.Name] = "no variable the type checker can determine"
		return
	}
	kind := ratchetMapKind(object.Type())
	if kind == "" {
		r.unsupported[name.Name] = "neither an integer constant nor a map[string]int or map[string]string register"
		return
	}
	for {
		paren, ok := expression.(*ast.ParenExpr)
		if !ok {
			break
		}
		expression = paren.X
	}
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		r.unsupported[name.Name] = "register is not a composite literal"
		return
	}
	entries := map[string]ratchetValue{}
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			r.unsupported[name.Name] = "register entry without a key"
			return
		}
		key := info.Types[pair.Key].Value
		if key == nil || key.Kind() != constant.String {
			r.unsupported[name.Name] = fmt.Sprintf("register key at %s is no string constant", r.fset.Position(pair.Pos()))
			return
		}
		number := 0
		if kind == "int" {
			value := info.Types[pair.Value].Value
			exact := false
			if value != nil && value.Kind() == constant.Int {
				var wide int64
				wide, exact = constant.Int64Val(value)
				number = int(wide)
			}
			if !exact {
				r.unsupported[name.Name] = fmt.Sprintf("register value at %s is no integer constant", r.fset.Position(pair.Pos()))
				return
			}
		}
		entries[constant.StringVal(key)] = ratchetValue{number, r.fset.Position(pair.Pos()).String(), commentText(file.comments, pair)}
	}
	location := r.fset.Position(name.Pos()).String()
	if kind == "int" {
		r.budgets[name.Name] = ratchetRegister{entries, location, doc}
		return
	}
	r.exceptions[name.Name] = ratchetValue{len(entries), location, doc}
}

// ratchetMapKind names the value kind of a register type: a map from a
// string type to an integer ("int") or string ("string") type.
func ratchetMapKind(typ types.Type) string {
	mapping, ok := typ.Underlying().(*types.Map)
	if !ok {
		return ""
	}
	key, ok := mapping.Key().Underlying().(*types.Basic)
	if !ok || key.Info()&types.IsString == 0 {
		return ""
	}
	value, ok := mapping.Elem().Underlying().(*types.Basic)
	switch {
	case !ok:
		return ""
	case value.Info()&types.IsInteger != 0:
		return "int"
	case value.Info()&types.IsString != 0:
		return "string"
	}
	return ""
}

// emptyImporter resolves every import to an empty package: the ratchet
// measurements are constants of the ratchet files themselves.
type emptyImporter struct{}

func (emptyImporter) Import(path string) (*types.Package, error) {
	pkg := types.NewPackage(path, filepath.Base(path))
	pkg.MarkComplete()
	return pkg, nil
}

func commentText(comments ast.CommentMap, node ast.Node) string {
	var text strings.Builder
	for _, group := range comments[node] {
		text.WriteString(group.Text())
	}
	return text.String()
}

var ratchetRenamedFrom = regexp.MustCompile(`(?i)\b(?:renamed|moved) from (\w+)`)

func renamedFrom(comment string) []string {
	var names []string
	for _, match := range ratchetRenamedFrom.FindAllStringSubmatch(comment, -1) {
		names = append(names, match[1])
	}
	return names
}

func compareRatchetBudgets(base, candidate *ratchetBudgets, adrs map[string]bool) []string {
	base.finalize()
	candidate.finalize()
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
	nameCited := func(name string) bool {
		pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
		for _, comment := range candidate.comments {
			if pattern.MatchString(comment) && justified(comment) {
				return true
			}
		}
		return false
	}
	// successors maps a base name to the one candidate declaration that says
	// it was renamed from it. Two claims on one name, or one declaration
	// claiming two names, are ambiguous and fail: a history moves to exactly
	// one successor.
	claims := map[string][]string{}
	claim := func(name, comment string) {
		predecessors := renamedFrom(comment)
		if len(predecessors) > 1 {
			raises = append(raises, fmt.Sprintf("%s names several predecessors (%s); a declaration continues one measurement", name, strings.Join(predecessors, ", ")))
		}
		for _, old := range predecessors {
			claims[old] = append(claims[old], name)
		}
	}
	for _, name := range sortedKeys(candidate.constants) {
		claim(name, candidate.constants[name].comment)
	}
	for _, name := range sortedKeys(candidate.budgets) {
		claim(name, candidate.budgets[name].comment)
	}
	for _, name := range sortedKeys(candidate.exceptions) {
		claim(name, candidate.exceptions[name].comment)
	}
	successors := map[string]string{}
	for _, old := range sortedKeys(claims) {
		names := claims[old]
		if len(names) > 1 {
			sort.Strings(names)
			raises = append(raises, fmt.Sprintf("%s is claimed by several successors (%s); mark exactly one \"renamed from %s\"", old, strings.Join(names, ", "), old))
			continue
		}
		successors[old] = names[0]
	}
	successor := func(name string) string {
		_, constant := candidate.constants[name]
		_, budget := candidate.budgets[name]
		_, exception := candidate.exceptions[name]
		_, unreadable := candidate.unsupported[name]
		if next, ok := successors[name]; ok && !constant && !budget && !exception && !unreadable {
			return next
		}
		return name
	}
	missing := func(kind, name, was string) {
		if reason, ok := candidate.unsupported[successor(name)]; ok {
			raises = append(raises, fmt.Sprintf("%s %s cannot be measured any more (%s); keep it a Go constant expression or register literal", kind, name, reason))
			return
		}
		if nameCited(name) {
			return
		}
		raises = append(raises, fmt.Sprintf("%s %s (%s at base) disappeared; keep it, mark its successor \"renamed from %s\" or cite an ADR this change adds", kind, name, was, name))
	}
	for _, name := range sortedKeys(base.constants) {
		was := base.constants[name]
		now, ok := candidate.constants[successor(name)]
		if !ok {
			missing("constant", name, strconv.Itoa(was.value))
			continue
		}
		if now.value > was.value && !justified(now.comment) {
			raises = append(raises, fmt.Sprintf("constant %s: %d -> %d at %s", name, was.value, now.value, now.location))
		}
	}
	for _, name := range sortedKeys(base.budgets) {
		register := base.budgets[name]
		before := 0
		for _, was := range register.entries {
			before += was.value
		}
		current, ok := candidate.budgets[successor(name)]
		if !ok {
			if before == 0 && candidate.unsupported[successor(name)] == "" {
				continue
			}
			missing("register", name, "sum "+strconv.Itoa(before))
			continue
		}
		after := 0
		for key, was := range register.entries {
			if now, ok := current.entries[key]; ok && now.value > was.value && !justified(now.comment, current.comment) {
				raises = append(raises, fmt.Sprintf("%s[%q]: %d -> %d at %s", name, key, was.value, now.value, now.location))
			}
		}
		for key, now := range current.entries {
			// An entry raised or added under an added ADR keeps its base share.
			if justified(now.comment, current.comment) {
				now = register.entries[key]
			}
			after += now.value
		}
		if after > before {
			raises = append(raises, fmt.Sprintf("%s: entries sum %d -> %d (a new or moved entry may not add to the register)", name, before, after))
		}
	}
	for _, name := range sortedKeys(base.exceptions) {
		was := base.exceptions[name]
		now, ok := candidate.exceptions[successor(name)]
		if !ok {
			if was.value == 0 && candidate.unsupported[successor(name)] == "" {
				continue
			}
			missing("exception register", name, strconv.Itoa(was.value)+" entries")
			continue
		}
		if now.value > was.value && !justified(now.comment) {
			raises = append(raises, fmt.Sprintf("%s: %d -> %d entries at %s", name, was.value, now.value, now.location))
		}
	}
	sort.Strings(raises)
	return raises
}

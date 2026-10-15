package architecture

import (
	"fmt"
	"go/ast"
	"go/constant"
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
// Values are Go constant expressions, evaluated with go/constant: literals,
// parentheses, operators, conversions, iota and references to other
// constants of the files. A register may have a named map type. A base
// measurement whose candidate declaration this evaluation cannot read is an
// error, never a silent drop. Other literals (selector sets, scanned
// directories) are configuration whose direction this comparison cannot judge.
//
// A declaration that is new in the candidate has no base and starts a
// measurement. A base measurement may not disappear: it stays, its register
// was empty, or a candidate declaration of the same kind says "renamed from
// <old name>" in its comment and is compared with the old base values. Moving
// a measurement out of the ratchet files therefore fails as well.
//
// A raise, or a disappearance, is accepted only when a comment on the raised
// entry or its declaration, or for a disappearance any comment naming the old
// declaration, cites an ADR ("ADR 0123") with a number the base does not have
// and the change adds as docs/adr/0123-*.md: a raised budget needs a new
// decision, and renaming an existing ADR file is none.
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

type ratchetConstant struct {
	expression ast.Expr
	iota       int64
	location   string
	comment    string
}

// finalize evaluates the measurements of all added files together, because
// a constant or a map type may be declared in another file of the package.
func (r *ratchetBudgets) finalize() {
	if r.finalized {
		return
	}
	r.finalized = true
	r.constants, r.budgets = map[string]ratchetValue{}, map[string]ratchetRegister{}
	r.exceptions, r.unsupported, r.comments = map[string]ratchetValue{}, map[string]string{}, nil
	declared := map[string]ratchetConstant{}
	typesByName := map[string]ast.Expr{}
	type variable struct {
		name    *ast.Ident
		typ     ast.Expr
		value   ast.Expr
		file    ratchetFile
		comment string
	}
	var variables []variable
	for _, file := range r.files {
		for _, group := range file.syntax.Comments {
			r.comments = append(r.comments, group.Text())
		}
		for _, declaration := range file.syntax.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			var previous []ast.Expr
			for index, spec := range general.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					typesByName[spec.Name.Name] = spec.Type
				case *ast.ValueSpec:
					doc := commentText(file.comments, general) + commentText(file.comments, spec)
					if general.Tok == token.CONST {
						values := spec.Values
						if len(values) == 0 {
							values = previous
						}
						previous = values
						for position, name := range spec.Names {
							if position < len(values) && name.Name != "_" {
								declared[name.Name] = ratchetConstant{values[position], int64(index), r.fset.Position(name.Pos()).String(), doc}
							}
						}
						continue
					}
					for position, name := range spec.Names {
						var value ast.Expr
						if position < len(spec.Values) {
							value = spec.Values[position]
						}
						variables = append(variables, variable{name, spec.Type, value, file, doc})
					}
				}
			}
		}
	}
	evaluator := &ratchetEvaluator{declared: declared, values: map[string]constant.Value{}, active: map[string]bool{}}
	for name, declaration := range declared {
		value, err := evaluator.constant(name)
		if err != nil {
			r.unsupported[name] = err.Error()
			continue
		}
		if value.Kind() != constant.Int {
			r.unsupported[name] = "no integer constant"
			continue
		}
		number, exact := constant.Int64Val(value)
		if !exact {
			r.unsupported[name] = "integer constant out of range"
			continue
		}
		r.constants[name] = ratchetValue{int(number), declaration.location, declaration.comment}
	}
	for _, variable := range variables {
		name := variable.name.Name
		value := variable.value
		for {
			paren, ok := value.(*ast.ParenExpr)
			if !ok {
				break
			}
			value = paren.X
		}
		literal, _ := value.(*ast.CompositeLit)
		typ := variable.typ
		if literal != nil && literal.Type != nil {
			typ = literal.Type
		}
		kind := ratchetMapValue(typ, typesByName)
		if kind == "" {
			// Not a register; named only for a base measurement that became it.
			r.unsupported[name] = "neither an integer constant nor a map[string]int or map[string]string literal"
			continue
		}
		location := r.fset.Position(variable.name.Pos()).String()
		if literal == nil {
			r.unsupported[name] = "register is not a composite literal"
			continue
		}
		entries := map[string]ratchetValue{}
		for _, element := range literal.Elts {
			pair, ok := element.(*ast.KeyValueExpr)
			if !ok {
				r.unsupported[name] = "register entry without a key"
				break
			}
			key, err := evaluator.expression(pair.Key, 0)
			if err != nil || key.Kind() != constant.String {
				r.unsupported[name] = fmt.Sprintf("register key at %s is no string constant", r.fset.Position(pair.Pos()))
				break
			}
			number := 0
			if kind == "int" {
				amount, err := evaluator.expression(pair.Value, 0)
				exact := false
				if err == nil && amount.Kind() == constant.Int {
					var wide int64
					wide, exact = constant.Int64Val(amount)
					number = int(wide)
				}
				if !exact {
					r.unsupported[name] = fmt.Sprintf("register value at %s is no integer constant", r.fset.Position(pair.Pos()))
					break
				}
			}
			entries[constant.StringVal(key)] = ratchetValue{number, r.fset.Position(pair.Pos()).String(), commentText(variable.file.comments, pair)}
		}
		if _, failed := r.unsupported[name]; failed {
			continue
		}
		if kind == "int" {
			r.budgets[name] = ratchetRegister{entries, location, variable.comment}
		} else {
			r.exceptions[name] = ratchetValue{len(entries), location, variable.comment}
		}
	}
}

// ratchetMapValue names the value kind of a map[string]int or
// map[string]string register type, following named types of the files.
func ratchetMapValue(expression ast.Expr, typesByName map[string]ast.Expr) string {
	resolved := ratchetResolveType(expression, typesByName, map[string]bool{})
	mapping, ok := resolved.(*ast.MapType)
	if !ok || ratchetBasicType(mapping.Key, typesByName) != "string" {
		return ""
	}
	switch value := ratchetBasicType(mapping.Value, typesByName); value {
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return "int"
	case "string":
		return "string"
	}
	return ""
}

func ratchetResolveType(expression ast.Expr, typesByName map[string]ast.Expr, seen map[string]bool) ast.Expr {
	switch typed := expression.(type) {
	case *ast.ParenExpr:
		return ratchetResolveType(typed.X, typesByName, seen)
	case *ast.Ident:
		if next, ok := typesByName[typed.Name]; ok && !seen[typed.Name] {
			seen[typed.Name] = true
			return ratchetResolveType(next, typesByName, seen)
		}
	}
	return expression
}

func ratchetBasicType(expression ast.Expr, typesByName map[string]ast.Expr) string {
	if id, ok := ratchetResolveType(expression, typesByName, map[string]bool{}).(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// ratchetEvaluator evaluates Go constant expressions over the constants the
// ratchet files declare.
type ratchetEvaluator struct {
	declared map[string]ratchetConstant
	values   map[string]constant.Value
	active   map[string]bool
}

func (e *ratchetEvaluator) constant(name string) (constant.Value, error) {
	if value, ok := e.values[name]; ok {
		return value, nil
	}
	declaration, ok := e.declared[name]
	if !ok {
		return nil, fmt.Errorf("unknown constant %s", name)
	}
	if e.active[name] {
		return nil, fmt.Errorf("constant %s refers to itself", name)
	}
	e.active[name] = true
	defer delete(e.active, name)
	value, err := e.expression(declaration.expression, declaration.iota)
	if err != nil {
		return nil, err
	}
	e.values[name] = value
	return value, nil
}

var ratchetConversions = map[string]bool{
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"uintptr": true, "byte": true, "rune": true, "string": true,
}

func (e *ratchetEvaluator) expression(expression ast.Expr, iota int64) (constant.Value, error) {
	switch typed := expression.(type) {
	case *ast.BasicLit:
		value := constant.MakeFromLiteral(typed.Value, typed.Kind, 0)
		if value.Kind() == constant.Unknown {
			return nil, fmt.Errorf("invalid literal %s", typed.Value)
		}
		return value, nil
	case *ast.ParenExpr:
		return e.expression(typed.X, iota)
	case *ast.Ident:
		switch typed.Name {
		case "iota":
			return constant.MakeInt64(iota), nil
		case "true", "false":
			return constant.MakeBool(typed.Name == "true"), nil
		}
		return e.constant(typed.Name)
	case *ast.UnaryExpr:
		operand, err := e.expression(typed.X, iota)
		if err != nil {
			return nil, err
		}
		switch typed.Op {
		case token.ADD, token.SUB, token.XOR, token.NOT:
			return constant.UnaryOp(typed.Op, operand, 0), nil
		}
	case *ast.BinaryExpr:
		left, err := e.expression(typed.X, iota)
		if err != nil {
			return nil, err
		}
		right, err := e.expression(typed.Y, iota)
		if err != nil {
			return nil, err
		}
		return ratchetBinary(typed.Op, left, right)
	case *ast.CallExpr:
		if function, ok := typed.Fun.(*ast.Ident); ok && ratchetConversions[function.Name] && len(typed.Args) == 1 {
			return e.expression(typed.Args[0], iota)
		}
	}
	return nil, fmt.Errorf("unsupported constant expression %T", expression)
}

func ratchetBinary(operator token.Token, left, right constant.Value) (constant.Value, error) {
	switch operator {
	case token.SHL, token.SHR:
		shift, exact := constant.Uint64Val(constant.ToInt(right))
		if !exact || shift > 1024 {
			return nil, fmt.Errorf("invalid shift count")
		}
		return constant.Shift(constant.ToInt(left), operator, uint(shift)), nil
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		return constant.MakeBool(constant.Compare(left, operator, right)), nil
	case token.QUO:
		if left.Kind() == constant.Int && right.Kind() == constant.Int {
			if constant.Sign(right) == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			operator = token.QUO_ASSIGN
		}
	case token.REM:
		if constant.Sign(right) == 0 {
			return nil, fmt.Errorf("division by zero")
		}
	}
	value := constant.BinaryOp(left, operator, right)
	if value.Kind() == constant.Unknown {
		return nil, fmt.Errorf("unsupported operator %s", operator)
	}
	return value, nil
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
	// successors maps a base name to the candidate declaration that says it
	// was renamed from it. A declaration names one predecessor at most.
	successors := map[string]string{}
	claim := func(name, comment string) {
		for _, old := range renamedFrom(comment) {
			if _, taken := successors[old]; !taken {
				successors[old] = name
			}
		}
	}
	for name, value := range candidate.constants {
		claim(name, value.comment)
	}
	for name, register := range candidate.budgets {
		claim(name, register.comment)
	}
	for name, value := range candidate.exceptions {
		claim(name, value.comment)
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
	for name, was := range base.constants {
		now, ok := candidate.constants[successor(name)]
		if !ok {
			missing("constant", name, strconv.Itoa(was.value))
			continue
		}
		if now.value > was.value && !justified(now.comment) {
			raises = append(raises, fmt.Sprintf("constant %s: %d -> %d at %s", name, was.value, now.value, now.location))
		}
	}
	for name, register := range base.budgets {
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
	for name, was := range base.exceptions {
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

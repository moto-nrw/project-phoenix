// Package test: calendar-date enforcement. See .claude/rules/calendar-dates.md.
//
// TestDateColumnTypes guarantees that every PostgreSQL DATE column maps to a
// timezone.Date model field, never time.Time. bun converts every time.Time
// parameter to UTC before binding, so a Berlin-midnight value stored through
// time.Time lands one day behind between 00:00 and 02:00 Berlin time — the
// root cause of ~20 production fixes in H1 2026.
//
// Mechanism: DATE columns are DISCOVERED by scanning the SQL inside
// database/migrations/*.go (never trust a hand-maintained list), then joined
// against model struct fields parsed from models/**/*.go. The allowlists
// below may only SHRINK — a stale entry fails the test and demands deletion.
//
// Known limitation: ad-hoc result structs inside database/repositories/
// (the ModelTableExpr pattern) are invisible to this test. Those are covered
// by the review checklist and detection greps in .claude/rules/calendar-dates.md.
package test

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// legacyTimeTimeDateColumns is the SHRINKING allowlist of DATE columns whose
// Go model field is still time.Time. NEVER add entries. Each migration
// commit deletes the rows it converts; the stale-entry sub-test forces the
// deletion. Seeded from the discovery scan when this test was introduced.
var legacyTimeTimeDateColumns = map[string]string{}

// unmappedDateColumns classifies DATE columns that have no models/ struct
// field (raw-SQL-only access or superseded tables). Every newly discovered
// unmapped column must be classified here with a reason.
var unmappedDateColumns = map[string]string{
	// Reminder push claims are written and deleted exclusively by the two
	// SECURITY DEFINER functions from 001015255; the occurrence date is bound as
	// a timezone.Date parameter there and never scanned into a struct, so the
	// table has no model to carry a field.
	"calendar.appointment_reminder_push_deliveries.occurrence_date": "claim table touched only through calendar.claim/release_appointment_reminder_push_delivery — no model struct",
}

// renamedDateColumns maps a DATE column declared under an old name in a
// migration to its current name after an ALTER TABLE ... RENAME COLUMN.
// The scanner only sees declarations; renames are direction-blind in
// migration+rollback strings, so they are recorded explicitly.
var renamedDateColumns = map[string]string{
	// 001015034_template_extensions.go renames enrollment_date to valid_from.
	"activities.student_enrollments.enrollment_date": "activities.student_enrollments.valid_from",
}

// droppedDateColumns lists DATE columns removed or retyped by a LATER
// forward migration (the scanner only sees declarations, not drops).
var droppedDateColumns = map[string]string{
	// Declared in the 001015068 ROLLBACK string; the forward migration drops
	// them from care_offerings — the phase owns the service window now.
	"enrollment.care_offerings.service_start_date": "dropped forward by 001015068 (phase owns the window)",
	"enrollment.care_offerings.service_end_date":   "dropped forward by 001015068 (phase owns the window)",
}

// truncate24hAllowlist holds files still containing Truncate(24 * time.Hour)
// date math (always wrong for Berlin calendar days). Shrink-only.
var truncate24hAllowlist = map[string]string{}

func TestDateColumnTypes(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	dateColumns := scanMigrationsForDateColumns(t, backendRoot)
	applyRenames(t, dateColumns)
	modelFields := scanModelDateFields(t, backendRoot, dateColumns)

	t.Run("date_columns_use_timezone_date", func(t *testing.T) {
		var violations []string
		for col, ref := range dateColumns {
			if _, dropped := droppedDateColumns[col]; dropped {
				continue
			}
			fields, mapped := modelFields[col]
			if !mapped {
				if _, ok := unmappedDateColumns[col]; !ok {
					violations = append(violations,
						"  unclassified DATE column "+col+" ("+ref+"):\n"+
							"    add a timezone.Date model field, or classify it in unmappedDateColumns/renamedDateColumns")
				}
				continue
			}
			for _, f := range fields {
				switch f.goType {
				case "timezone.Date", "*timezone.Date":
					// migrated — ok
				case "time.Time", "*time.Time":
					if _, ok := legacyTimeTimeDateColumns[col]; !ok {
						violations = append(violations, formatViolation(f.file, f.line,
							col+" is a DATE column but "+f.structName+"."+f.fieldName+" is "+f.goType+
								" — use timezone.Date (see .claude/rules/calendar-dates.md)"))
					}
				default:
					violations = append(violations, formatViolation(f.file, f.line,
						col+" maps to unexpected Go type "+f.goType+" — use timezone.Date"))
				}
			}
		}
		if len(violations) > 0 {
			t.Errorf("Found %d DATE-column type violation(s):\n\n%s\n\n"+
				"time.Time bound to a DATE column is converted to UTC by bun and lands one day\n"+
				"behind between 00:00 and 02:00 Berlin time. Use timezone.Date.",
				len(violations), strings.Join(violations, "\n"))
		}
	})

	t.Run("allowlist_entries_still_required", func(t *testing.T) {
		var stale []string
		for col := range legacyTimeTimeDateColumns {
			fields, mapped := modelFields[col]
			_, exists := dateColumns[col]
			if !exists || !mapped || !anyTimeTime(fields) {
				stale = append(stale, "  legacyTimeTimeDateColumns: "+col)
			}
		}
		for col := range unmappedDateColumns {
			_, mapped := modelFields[col]
			_, exists := dateColumns[col]
			if !exists || mapped {
				stale = append(stale, "  unmappedDateColumns: "+col)
			}
		}
		for oldCol, newCol := range renamedDateColumns {
			if _, oldStillDeclared := dateColumns[oldCol]; oldStillDeclared {
				// The rename consumed the old name; finding it again means the
				// scanner state changed — re-verify the map entry.
				stale = append(stale, "  renamedDateColumns (old name re-appeared): "+oldCol)
			}
			if _, newKnown := dateColumns[newCol]; !newKnown {
				stale = append(stale, "  renamedDateColumns (new name unknown): "+newCol)
			}
		}
		if len(stale) > 0 {
			t.Errorf("Stale allowlist entries (column migrated, remapped, or gone) — DELETE them:\n%s",
				strings.Join(stale, "\n"))
		}
	})

	t.Run("no_truncate_24h_date_math", func(t *testing.T) {
		truncRe := regexp.MustCompile(`\.Truncate\(\s*24\s*\*\s*time\.Hour\s*\)`)
		violationsByFile := map[string][]string{}

		err := filepath.Walk(backendRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			rel, _ := filepath.Rel(backendRoot, path)
			if rel == filepath.Join("test", "calendar_date_verification_test.go") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for i, line := range strings.Split(string(data), "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				if truncRe.MatchString(line) {
					violationsByFile[rel] = append(violationsByFile[rel],
						formatViolation(rel, i+1, strings.TrimSpace(line)))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking backend root: %v", err)
		}

		var violations, stale []string
		for file, vs := range violationsByFile {
			if _, allowed := truncate24hAllowlist[file]; !allowed {
				violations = append(violations, vs...)
			}
		}
		for file := range truncate24hAllowlist {
			if _, found := violationsByFile[file]; !found {
				stale = append(stale, "  truncate24hAllowlist: "+file)
			}
		}
		if len(violations) > 0 {
			t.Errorf("Found %d Truncate(24 * time.Hour) occurrence(s) — UTC-day math, not Berlin "+
				"calendar days. Use timezone.Date / timezone.TodayDate():\n\n%s",
				len(violations), strings.Join(violations, "\n"))
		}
		if len(stale) > 0 {
			t.Errorf("Stale truncate24hAllowlist entries — DELETE them:\n%s", strings.Join(stale, "\n"))
		}
	})
}

// --- migration scanning ---

var (
	createTableRe = regexp.MustCompile(`CREATE TABLE (?:IF NOT EXISTS )?([a-z_]+\.[a-z_]+)`)
	alterTableRe  = regexp.MustCompile(`ALTER TABLE (?:ONLY )?(?:IF EXISTS )?([a-z_]+\.[a-z_]+)`)
	// Case-sensitive DATE so CURRENT_DATE, ::date casts, and VALIDATE never match.
	colDeclRe = regexp.MustCompile(`^\s*([a-z_]+)\s+DATE\b`)
	addColRe  = regexp.MustCompile(`ADD COLUMN\s+(?:IF NOT EXISTS\s+)?([a-z_]+)\s+DATE\b`)
)

// scanMigrationsForDateColumns line-scans the SQL embedded in migration Go
// files and returns map["schema.table.column"]"file:line" of every DATE
// column declaration.
func scanMigrationsForDateColumns(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}

	migrationsDir := filepath.Join(root, "database", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("reading migrations dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(migrationsDir, entry.Name())
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("opening %s: %v", path, err)
		}

		currentTable := ""
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "--") {
				continue
			}

			if m := createTableRe.FindStringSubmatch(line); m != nil {
				currentTable = m[1]
			} else if m := alterTableRe.FindStringSubmatch(line); m != nil {
				currentTable = m[1]
			}
			if currentTable == "" {
				continue
			}

			ref := "database/migrations/" + entry.Name() + ":" + itoa(lineNo)
			if m := addColRe.FindStringSubmatch(line); m != nil {
				result[currentTable+"."+m[1]] = ref
			} else if m := colDeclRe.FindStringSubmatch(line); m != nil {
				result[currentTable+"."+m[1]] = ref
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatalf("scanning %s: %v", path, err)
		}
		_ = f.Close()
	}
	return result
}

// applyRenames replaces old column names with their current names so the
// model join uses today's schema.
func applyRenames(t *testing.T, dateColumns map[string]string) {
	t.Helper()
	for oldCol, newCol := range renamedDateColumns {
		if ref, ok := dateColumns[oldCol]; ok {
			delete(dateColumns, oldCol)
			dateColumns[newCol] = ref + " (renamed)"
		}
	}
}

// --- model scanning ---

type dateFieldInfo struct {
	file       string
	line       int
	structName string
	fieldName  string
	goType     string
}

// scanModelDateFields parses every struct in models/**/*.go and returns
// map["schema.table.column"][]dateFieldInfo for all fields. Fields tagged
// type:date additionally register their column in dateColumns (belt and
// braces for columns the migration scan cannot see).
func scanModelDateFields(t *testing.T, root string, dateColumns map[string]string) map[string][]dateFieldInfo {
	t.Helper()
	result := map[string][]dateFieldInfo{}
	modelsDir := filepath.Join(root, "models")
	fset := token.NewFileSet()

	err := filepath.Walk(modelsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr != nil {
			return parseErr
		}
		rel, _ := filepath.Rel(root, path)
		methodTables := tableNamesFromMethods(file)

		ast.Inspect(file, func(n ast.Node) bool {
			typeSpec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return true
			}

			table := structTableName(structType)
			if table == "" {
				table = methodTables[typeSpec.Name.Name]
			}
			if table == "" {
				return true
			}

			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue // embedded
				}
				column, bunTag := fieldColumn(field)
				if column == "" {
					continue
				}
				key := table + "." + column
				goType := renderType(field.Type)
				fi := dateFieldInfo{
					file:       rel,
					line:       fset.Position(field.Pos()).Line,
					structName: typeSpec.Name.Name,
					fieldName:  field.Names[0].Name,
					goType:     goType,
				}
				result[key] = append(result[key], fi)
				if strings.Contains(bunTag, "type:date") {
					if _, known := dateColumns[key]; !known {
						dateColumns[key] = rel + ":" + itoa(fi.line) + " (type:date tag)"
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking models dir: %v", err)
	}
	return result
}

// tableNamesFromMethods maps struct names to "schema.table" for structs that
// declare their table only via a TableName() method (no embedded bun tag —
// e.g. config.StaffWorkSchedule). Resolves both string literals and
// file-level string constants.
func tableNamesFromMethods(file *ast.File) map[string]string {
	dotted := regexp.MustCompile(`^[a-z_]+\.[a-z_]+$`)

	consts := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != len(vs.Values) {
				continue
			}
			for i, name := range vs.Names {
				if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					consts[name.Name] = strings.Trim(lit.Value, "`\"")
				}
			}
		}
	}

	result := map[string]string{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "TableName" || fn.Recv == nil || len(fn.Recv.List) != 1 || fn.Body == nil {
			continue
		}
		recvType := fn.Recv.List[0].Type
		if star, ok := recvType.(*ast.StarExpr); ok {
			recvType = star.X
		}
		recvName, ok := recvType.(*ast.Ident)
		if !ok {
			continue
		}
		for _, stmt := range fn.Body.List {
			ret, ok := stmt.(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				continue
			}
			var value string
			switch r := ret.Results[0].(type) {
			case *ast.BasicLit:
				if r.Kind == token.STRING {
					value = strings.Trim(r.Value, "`\"")
				}
			case *ast.Ident:
				value = consts[r.Name]
			}
			if dotted.MatchString(value) {
				result[recvName.Name] = value
			}
		}
	}
	return result
}

// structTableName extracts "schema.table" from embedded-field bun tags:
// `bun:"schema:active,table:student_status_days"` (on base.Model) or
// `bun:"active.student_status_days"` / `bun:"table:active.x,alias:y"`
// (on bun.BaseModel).
func structTableName(s *ast.StructType) string {
	schemaTableRe := regexp.MustCompile(`schema:([a-z_]+),table:([a-z_]+)`)
	dottedTableRe := regexp.MustCompile(`(?:^|table:)([a-z_]+\.[a-z_]+)`)

	for _, field := range s.Fields.List {
		if len(field.Names) != 0 || field.Tag == nil {
			continue
		}
		tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`")).Get("bun")
		if tag == "" {
			continue
		}
		if m := schemaTableRe.FindStringSubmatch(tag); m != nil {
			return m[1] + "." + m[2]
		}
		if m := dottedTableRe.FindStringSubmatch(strings.Split(tag, ",")[0]); m != nil {
			return m[1]
		}
		if m := dottedTableRe.FindStringSubmatch(tag); m != nil {
			return m[1]
		}
	}
	return ""
}

// fieldColumn returns the DB column name for a struct field and the raw bun
// tag. Explicit bun tag column wins; otherwise bun's default underscored
// field name applies.
func fieldColumn(field *ast.Field) (string, string) {
	bunTag := ""
	if field.Tag != nil {
		bunTag = reflect.StructTag(strings.Trim(field.Tag.Value, "`")).Get("bun")
	}
	if bunTag == "-" {
		return "", bunTag
	}
	first := strings.Split(bunTag, ",")[0]
	if first != "" && !strings.Contains(first, ":") {
		return first, bunTag
	}
	if strings.Contains(bunTag, "rel:") || strings.Contains(bunTag, "join:") || strings.Contains(bunTag, "m2m:") {
		return "", bunTag
	}
	return underscore(field.Names[0].Name), bunTag
}

// renderType renders the small set of type expressions we care about.
func renderType(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return "*" + renderType(e.X)
	case *ast.SelectorExpr:
		if pkg, ok := e.X.(*ast.Ident); ok {
			return pkg.Name + "." + e.Sel.Name
		}
		return e.Sel.Name
	case *ast.Ident:
		return e.Name
	default:
		return "(complex)"
	}
}

// underscore approximates bun's CamelCase → snake_case column naming.
func underscore(name string) string {
	var b strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			if i > 0 && (isLower(runes[i-1]) || (i+1 < len(runes) && isLower(runes[i+1]))) {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isLower(r rune) bool { return r >= 'a' && r <= 'z' }

func anyTimeTime(fields []dateFieldInfo) bool {
	for _, f := range fields {
		if f.goType == "time.Time" || f.goType == "*time.Time" {
			return true
		}
	}
	return false
}

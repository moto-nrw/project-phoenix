package common_test

import (
	"fmt"
	"go/ast"
	"go/token"
	"net/http"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	netHTTPPackagePath   = "net/http"
	chiRenderPackagePath = "github.com/go-chi/render"
)

// TestErrorWritesUseTheSharedEnvelope fails on an error write in backend
// source that bypasses the shared error envelope (ADR 0006, #2520). It
// complements TestErrorPathsAnswerWithRegisteredCodes, which checks the
// codes themselves, and checkErrorEnvelopeOnEveryRoute, which proves the
// envelope on every route at run time. It has no exception list; every rule
// is about a kind of construct:
//
//   - http.Error and http.NotFound write a plain-text body. Render an
//     ErrResponse through api/common instead, or, in a package that may not
//     import it, the package's own envelope.
//   - An error body type is a struct whose Render method sets the response
//     status (render.Status), that embeds common.ErrResponse, or that
//     declares `detail` next to `code` or `status`. It embeds
//     common.ErrResponse or declares `status` as the HTTP status number,
//     `detail` and `code` as strings, and never `error` or `message`. The
//     codes its literals name must be registered.
//   - A struct with an `error` member next to `code`, or a map literal with an
//     "error" or "message" key next to "code", "detail" or an error status
//     ("error" or an http.Status* constant >= 400), is the pre-#2520
//     envelope written by hand. An `error` member alone is not:
//     per-row results of a batch answer carry one inside a success body.
//   - A map literal that is an error body (a "detail" key or an error
//     status) needs a "code"; an error body struct (see above) needs one too.
//   - WriteHeader(<http.Status* constant >= 400>) followed by a raw write
//     (Write, fmt.Fprint*, io.WriteString) in the same function writes an
//     error body outside the envelope.
//
// Two constructs are deliberately outside the scan. A common.ErrResponse
// literal without Code is fine: its Render fills the class code of the
// status, and every class code is registered (#2503). A code that is only
// known at run time, such as a variable, is out of reach of a static scan;
// the route walk of checkErrorEnvelopeOnEveryRoute checks those answers.
func TestErrorWritesUseTheSharedEnvelope(t *testing.T) {
	t.Parallel()

	violations := scanErrorEnvelopes(t, backendRoot(t), loadErrorCodeRegistry(t))
	if len(violations) > 0 {
		t.Errorf("%d error write(s) outside the shared envelope. Render the failure through api/common "+
			"(RenderError, RenderStatus) or the package's envelope with a registered code:\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

// TestErrorEnvelopeScanFindsEachViolation runs the scan over a fixture tree
// with one violation of each kind, so a scan that silently finds nothing
// fails.
func TestErrorEnvelopeScanFindsEachViolation(t *testing.T) {
	t.Parallel()

	root := filepath.Join("testdata", "errorenvelope")
	violations := scanErrorEnvelopes(t, root, loadErrorCodeRegistry(t))
	for index, violation := range violations {
		violations[index] = filepath.ToSlash(strings.TrimPrefix(violation, root+string(filepath.Separator)))
	}
	want := []string{
		`api/bad/bad.go:14:2: http.Error writes a plain-text error body outside the shared envelope`,
		`api/bad/bad.go:15:2: http.NotFound writes a plain-text error body outside the shared envelope`,
		`api/bad/bad.go:19:6: error body legacyBody: status must be the HTTP status number (int), not string`,
		`api/bad/bad.go:19:6: error body legacyBody: carries the legacy "error" member; the text belongs in "detail"`,
		`api/bad/bad.go:19:6: error body legacyBody: has no "code" member`,
		`api/bad/bad.go:19:6: error body legacyBody: has no "detail" member`,
		`api/bad/bad.go:30:6: error body messageBody: carries the legacy "message" member; the text belongs in "detail"`,
		`api/bad/bad.go:42:71: error body problem code is "care.not_registered", which is not in error-registry/`,
		`api/bad/bad.go:45:9: map literal writes the legacy "error" member next to an error status; render the shared envelope`,
		`api/bad/bad.go:49:6: struct rowFailure writes the legacy "error" member next to "code"; render the shared envelope`,
		`api/bad/bad.go:67:9: map literal writes the legacy "message" member next to an error status; render the shared envelope`,
		`api/bad/bad.go:71:9: map literal writes an error body without a "code"; render the shared envelope`,
		`api/bad/bad.go:75:9: map literal writes an error body without a "code"; render the shared envelope`,
		`api/bad/bad.go:84:2: WriteHeader with an error status is followed by a raw body; render the shared envelope`,
		`api/bad/bad.go:94:6: error body titledProblem: has no "code" member`,
	}
	sort.Strings(want)
	if strings.Join(violations, "\n") != strings.Join(want, "\n") {
		t.Errorf("scan found:\n%s\n\nwant:\n%s", strings.Join(violations, "\n"), strings.Join(want, "\n"))
	}
}

// envelopeMember is one JSON member of a struct: its name and Go type.
type envelopeMember struct {
	typeName string
	pos      token.Pos
}

// structShape is a struct type with its JSON members, the embedded
// ErrResponse marker and whether its Render sets an error status.
type structShape struct {
	name         string
	pos          token.Pos
	members      map[string]envelopeMember
	embedsShared bool
	embedded     []string // same-package struct types embedded by value or pointer
	setsStatus   bool
}

func scanErrorEnvelopes(t *testing.T, root string, registry errorCodeRegistry) []string {
	t.Helper()
	tree := loadSourceTree(t, root)
	statuses := httpStatusConstants(t)
	violationAt := map[string]bool{}
	var violations []string
	report := func(pos token.Pos, format string, args ...any) {
		violation := tree.fset.Position(pos).String() + ": " + fmt.Sprintf(format, args...)
		if !violationAt[violation] {
			violationAt[violation] = true
			violations = append(violations, violation)
		}
	}
	for _, pkg := range tree.packages {
		shapes := tree.structShapes(pkg)
		bodies := map[string]bool{}
		for name, shape := range shapes {
			if isErrorBody(shapes, shape) {
				bodies[name] = true
				checkErrorBody(shapes, shape, report)
			} else if _, ok := shape.members["error"]; ok && hasMember(shapes, shape, "code", 0) {
				report(shape.pos, "struct %s writes the legacy \"error\" member next to \"code\"; render the shared envelope", name)
			}
		}
		for _, file := range pkg.files {
			if ast.IsGenerated(file.syntax) {
				continue
			}
			tree.checkEnvelopeFile(pkg, file, bodies, statuses, registry, report)
			checkRawErrorWrites(file, statuses, report)
		}
	}
	sort.Strings(violations)
	return violations
}

func (tree *sourceTree) checkEnvelopeFile(pkg *sourcePackage, file *sourceFile, bodies map[string]bool, statuses map[string]int, registry errorCodeRegistry, report func(token.Pos, string, ...any)) {
	ast.Inspect(file.syntax, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CallExpr:
			if name, ok := importedName(file, n.Fun, netHTTPPackagePath); ok && (name == "Error" || name == "NotFound") {
				report(n.Pos(), "http.%s writes a plain-text error body outside the shared envelope", name)
			}
		case *ast.CompositeLit:
			if ident, ok := ast.Unparen(n.Type).(*ast.Ident); ok && bodies[ident.Name] {
				for _, element := range n.Elts {
					field, ok := element.(*ast.KeyValueExpr)
					if !ok || identName(field.Key) != "Code" {
						continue
					}
					tree.reportCode(pkg, file, field.Value, "error body "+ident.Name+" code", registry, report)
				}
			}
			if _, isMap := n.Type.(*ast.MapType); isMap {
				checkMapBody(file, n, statuses, report)
			}
		}
		return true
	})
}

// reportCode reports a code value that is a string at the call site or that
// resolves to an unregistered code, like the registry scan does.
func (tree *sourceTree) reportCode(pkg *sourcePackage, file *sourceFile, expr ast.Expr, where string, registry errorCodeRegistry, report func(token.Pos, string, ...any)) {
	if value, ok := tree.resolve(pkg, file, expr, 0); ok && !registry.codes[value] {
		report(expr.Pos(), "%s is %q, which is not in error-registry/", where, value)
	}
	for _, value := range tree.returnedStrings(pkg, file, expr) {
		if !registry.codes[value] {
			report(expr.Pos(), "%s can be %q, which is not in error-registry/", where, value)
		}
	}
}

// isErrorBody reports a struct that answers an error: its Render sets the
// response status, it embeds the shared envelope, or it declares a `detail`
// member next to `code` or `status`. A `detail` member alone is not one;
// audit events carry it as a description.
func isErrorBody(shapes map[string]*structShape, shape *structShape) bool {
	if shape.setsStatus || embedsShared(shapes, shape, 0) {
		return true
	}
	return hasMember(shapes, shape, "detail", 0) && (hasMember(shapes, shape, "code", 0) || hasMember(shapes, shape, "status", 0))
}

// checkMapBody flags a map literal that writes an error body by hand: an
// `error` or `message` text next to `code`, `detail` or an error status
// ("error" or an http.Status* constant >= 400), or an error body (a
// `detail` or an error status) without a `code`. A `message` next to a
// business status is a success payload and stays.
func checkMapBody(file *sourceFile, lit *ast.CompositeLit, statuses map[string]int, report func(token.Pos, string, ...any)) {
	keys := map[string]ast.Expr{}
	for _, element := range lit.Elts {
		if field, ok := element.(*ast.KeyValueExpr); ok {
			if key, ok := stringLiteral(field.Key); ok {
				keys[key] = field.Value
			}
		}
	}
	status, hasStatus := keys["status"]
	errorStatus := hasStatus && (isHTTPErrorStatus(file, status, statuses) || isStringLiteral(status, "error"))
	for _, legacy := range []string{"error", "message"} {
		if _, ok := keys[legacy]; !ok {
			continue
		}
		for _, neighbour := range []string{"code", "detail"} {
			if _, ok := keys[neighbour]; ok {
				report(lit.Pos(), "map literal writes the legacy %q member next to %q; render the shared envelope", legacy, neighbour)
				return
			}
		}
		if errorStatus {
			report(lit.Pos(), "map literal writes the legacy %q member next to an error status; render the shared envelope", legacy)
			return
		}
	}
	if _, hasCode := keys["code"]; hasCode {
		return
	}
	_, hasDetail := keys["detail"]
	if hasDetail || errorStatus {
		report(lit.Pos(), "map literal writes an error body without a \"code\"; render the shared envelope")
	}
}

func isStringLiteral(expr ast.Expr, want string) bool {
	value, ok := stringLiteral(expr)
	return ok && value == want
}

// isHTTPErrorStatus reports an http.Status* constant of 400 or above.
func isHTTPErrorStatus(file *sourceFile, expr ast.Expr, statuses map[string]int) bool {
	name, ok := importedName(file, expr, netHTTPPackagePath)
	return ok && statuses[name] >= 400
}

func checkErrorBody(shapes map[string]*structShape, shape *structShape, report func(token.Pos, string, ...any)) {
	for _, legacy := range []string{"error", "message"} {
		if hasMember(shapes, shape, legacy, 0) {
			report(shape.pos, "error body %s: carries the legacy %q member; the text belongs in \"detail\"", shape.name, legacy)
		}
	}
	if embedsShared(shapes, shape, 0) {
		return
	}
	if member, ok := findMember(shapes, shape, "status", 0); !ok {
		report(shape.pos, "error body %s: has no \"status\" member", shape.name)
	} else if member.typeName != "int" {
		report(shape.pos, "error body %s: status must be the HTTP status number (int), not %s", shape.name, member.typeName)
	}
	for _, required := range []string{"detail", "code"} {
		if member, ok := findMember(shapes, shape, required, 0); !ok {
			report(shape.pos, "error body %s: has no %q member", shape.name, required)
		} else if member.typeName != "string" {
			report(shape.pos, "error body %s: %s must be a string, not %s", shape.name, required, member.typeName)
		}
	}
}

func hasMember(shapes map[string]*structShape, shape *structShape, name string, depth int) bool {
	_, ok := findMember(shapes, shape, name, depth)
	return ok
}

// findMember looks a JSON member up in the struct and the same-package
// structs it embeds.
func findMember(shapes map[string]*structShape, shape *structShape, name string, depth int) (envelopeMember, bool) {
	if member, ok := shape.members[name]; ok {
		return member, true
	}
	if depth > 5 {
		return envelopeMember{}, false
	}
	for _, embedded := range shape.embedded {
		if inner, ok := shapes[embedded]; ok {
			if member, ok := findMember(shapes, inner, name, depth+1); ok {
				return member, true
			}
		}
	}
	return envelopeMember{}, false
}

func embedsShared(shapes map[string]*structShape, shape *structShape, depth int) bool {
	if shape.embedsShared {
		return true
	}
	if depth > 5 {
		return false
	}
	for _, embedded := range shape.embedded {
		if inner, ok := shapes[embedded]; ok && embedsShared(shapes, inner, depth+1) {
			return true
		}
	}
	return false
}

// structShapes collects the package's struct types with their JSON members
// and marks the ones whose Render method sets the response status.
func (tree *sourceTree) structShapes(pkg *sourcePackage) map[string]*structShape {
	shapes := map[string]*structShape{}
	inCommon := pkg.path == commonPackagePath
	for _, file := range pkg.files {
		for _, decl := range file.syntax.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				shape := &structShape{name: typeSpec.Name.Name, pos: typeSpec.Name.Pos(), members: map[string]envelopeMember{}}
				for _, field := range structType.Fields.List {
					tree.addField(file, inCommon, shape, field)
				}
				shapes[shape.name] = shape
			}
		}
	}
	for _, file := range pkg.files {
		for _, decl := range file.syntax.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Recv == nil || function.Name.Name != "Render" || function.Body == nil || len(function.Type.Params.List) != 2 {
				continue
			}
			if shape, ok := shapes[receiverName(function)]; ok && callsRenderStatus(file, function.Body) {
				shape.setsStatus = true
			}
		}
	}
	return shapes
}

func (tree *sourceTree) addField(file *sourceFile, inCommon bool, shape *structShape, field *ast.Field) {
	fieldType := field.Type
	if star, ok := fieldType.(*ast.StarExpr); ok {
		fieldType = star.X
	}
	if len(field.Names) == 0 {
		if name, ok := tree.commonName(file, inCommon, fieldType); ok && name == "ErrResponse" {
			shape.embedsShared = true
		} else if ident, ok := fieldType.(*ast.Ident); ok {
			shape.embedded = append(shape.embedded, ident.Name)
		}
		return
	}
	member := jsonName(field)
	if member == "" || member == "-" {
		return
	}
	typeName := "other"
	if ident, ok := fieldType.(*ast.Ident); ok {
		typeName = ident.Name
	}
	shape.members[member] = envelopeMember{typeName: typeName, pos: field.Pos()}
}

func jsonName(field *ast.Field) string {
	if field.Tag == nil {
		return ""
	}
	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return ""
	}
	name, _, _ := strings.Cut(reflect.StructTag(tag).Get("json"), ",")
	return name
}

func receiverName(function *ast.FuncDecl) string {
	if len(function.Recv.List) == 0 {
		return ""
	}
	receiver := function.Recv.List[0].Type
	if star, ok := receiver.(*ast.StarExpr); ok {
		receiver = star.X
	}
	return identName(receiver)
}

func callsRenderStatus(file *sourceFile, body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			if name, ok := importedName(file, call.Fun, chiRenderPackagePath); ok && name == "Status" {
				found = true
			}
		}
		return !found
	})
	return found
}

// checkRawErrorWrites flags a function that sets an error status with
// WriteHeader(<http.Status* constant >= 400>) and then writes a raw body
// (Write, fmt.Fprint*, io.WriteString) instead of an envelope. A
// WriteHeader with a variable status is out of reach of a static scan; the
// route walk and ProblemResponseMiddleware cover it.
func checkRawErrorWrites(file *sourceFile, statuses map[string]int, report func(token.Pos, string, ...any)) {
	ast.Inspect(file.syntax, func(node ast.Node) bool {
		var body *ast.BlockStmt
		switch n := node.(type) {
		case *ast.FuncDecl:
			body = n.Body
		case *ast.FuncLit:
			body = n.Body
		}
		if body == nil {
			return true
		}
		var errorStatus *ast.CallExpr
		rawWrite := false
		ast.Inspect(body, func(inner ast.Node) bool {
			if _, nested := inner.(*ast.FuncLit); nested {
				return false
			}
			call, ok := inner.(*ast.CallExpr)
			if !ok {
				return true
			}
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) == 1 {
				switch selector.Sel.Name {
				case "WriteHeader":
					if name, ok := importedName(file, call.Args[0], netHTTPPackagePath); ok && statuses[name] >= 400 && errorStatus == nil {
						errorStatus = call
					}
				case "Write":
					rawWrite = rawWrite || errorStatus != nil
				}
			}
			if name, ok := importedName(file, call.Fun, "fmt"); ok && strings.HasPrefix(name, "Fprint") && errorStatus != nil {
				rawWrite = true
			}
			if name, ok := importedName(file, call.Fun, "io"); ok && name == "WriteString" && errorStatus != nil {
				rawWrite = true
			}
			return true
		})
		if errorStatus != nil && rawWrite {
			report(errorStatus.Pos(), "WriteHeader with an error status is followed by a raw body; render the shared envelope")
		}
		return true
	})
}

// httpStatusConstants maps every http.Status* constant name to its value.
// The names compile against net/http, and the completeness check below
// fails when net/http gains a status the table does not name.
func httpStatusConstants(t *testing.T) map[string]int {
	t.Helper()
	statuses := map[string]int{
		"StatusContinue":                      http.StatusContinue,
		"StatusSwitchingProtocols":            http.StatusSwitchingProtocols,
		"StatusProcessing":                    http.StatusProcessing,
		"StatusEarlyHints":                    http.StatusEarlyHints,
		"StatusOK":                            http.StatusOK,
		"StatusCreated":                       http.StatusCreated,
		"StatusAccepted":                      http.StatusAccepted,
		"StatusNonAuthoritativeInfo":          http.StatusNonAuthoritativeInfo,
		"StatusNoContent":                     http.StatusNoContent,
		"StatusResetContent":                  http.StatusResetContent,
		"StatusPartialContent":                http.StatusPartialContent,
		"StatusMultiStatus":                   http.StatusMultiStatus,
		"StatusAlreadyReported":               http.StatusAlreadyReported,
		"StatusIMUsed":                        http.StatusIMUsed,
		"StatusMultipleChoices":               http.StatusMultipleChoices,
		"StatusMovedPermanently":              http.StatusMovedPermanently,
		"StatusFound":                         http.StatusFound,
		"StatusSeeOther":                      http.StatusSeeOther,
		"StatusNotModified":                   http.StatusNotModified,
		"StatusUseProxy":                      http.StatusUseProxy,
		"StatusTemporaryRedirect":             http.StatusTemporaryRedirect,
		"StatusPermanentRedirect":             http.StatusPermanentRedirect,
		"StatusBadRequest":                    http.StatusBadRequest,
		"StatusUnauthorized":                  http.StatusUnauthorized,
		"StatusPaymentRequired":               http.StatusPaymentRequired,
		"StatusForbidden":                     http.StatusForbidden,
		"StatusNotFound":                      http.StatusNotFound,
		"StatusMethodNotAllowed":              http.StatusMethodNotAllowed,
		"StatusNotAcceptable":                 http.StatusNotAcceptable,
		"StatusProxyAuthRequired":             http.StatusProxyAuthRequired,
		"StatusRequestTimeout":                http.StatusRequestTimeout,
		"StatusConflict":                      http.StatusConflict,
		"StatusGone":                          http.StatusGone,
		"StatusLengthRequired":                http.StatusLengthRequired,
		"StatusPreconditionFailed":            http.StatusPreconditionFailed,
		"StatusRequestEntityTooLarge":         http.StatusRequestEntityTooLarge,
		"StatusRequestURITooLong":             http.StatusRequestURITooLong,
		"StatusUnsupportedMediaType":          http.StatusUnsupportedMediaType,
		"StatusRequestedRangeNotSatisfiable":  http.StatusRequestedRangeNotSatisfiable,
		"StatusExpectationFailed":             http.StatusExpectationFailed,
		"StatusTeapot":                        http.StatusTeapot,
		"StatusMisdirectedRequest":            http.StatusMisdirectedRequest,
		"StatusUnprocessableEntity":           http.StatusUnprocessableEntity,
		"StatusLocked":                        http.StatusLocked,
		"StatusFailedDependency":              http.StatusFailedDependency,
		"StatusTooEarly":                      http.StatusTooEarly,
		"StatusUpgradeRequired":               http.StatusUpgradeRequired,
		"StatusPreconditionRequired":          http.StatusPreconditionRequired,
		"StatusTooManyRequests":               http.StatusTooManyRequests,
		"StatusRequestHeaderFieldsTooLarge":   http.StatusRequestHeaderFieldsTooLarge,
		"StatusUnavailableForLegalReasons":    http.StatusUnavailableForLegalReasons,
		"StatusInternalServerError":           http.StatusInternalServerError,
		"StatusNotImplemented":                http.StatusNotImplemented,
		"StatusBadGateway":                    http.StatusBadGateway,
		"StatusServiceUnavailable":            http.StatusServiceUnavailable,
		"StatusGatewayTimeout":                http.StatusGatewayTimeout,
		"StatusHTTPVersionNotSupported":       http.StatusHTTPVersionNotSupported,
		"StatusVariantAlsoNegotiates":         http.StatusVariantAlsoNegotiates,
		"StatusInsufficientStorage":           http.StatusInsufficientStorage,
		"StatusLoopDetected":                  http.StatusLoopDetected,
		"StatusNotExtended":                   http.StatusNotExtended,
		"StatusNetworkAuthenticationRequired": http.StatusNetworkAuthenticationRequired,
	}
	named := map[int]bool{}
	for _, value := range statuses {
		named[value] = true
	}
	for code := 100; code <= 599; code++ {
		if http.StatusText(code) != "" && !named[code] {
			t.Fatalf("net/http knows status %d (%s); add its constant to httpStatusConstants", code, http.StatusText(code))
		}
	}
	return statuses
}

// importedName returns Name for a call target alias.Name whose alias the
// file imports from importPath.
func importedName(file *sourceFile, fun ast.Expr, importPath string) (string, bool) {
	selector, ok := ast.Unparen(fun).(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	alias, ok := selector.X.(*ast.Ident)
	if !ok || file.imports[alias.Name] != importPath {
		return "", false
	}
	return selector.Sel.Name, true
}

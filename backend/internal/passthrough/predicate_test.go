package passthrough

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestPassthroughPredicateFixtures(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct {
		name, method string
		want         bool
	}{
		{"forward", "func (s *Service) Find() { s.store.Find() }", true},
		{"unexported", "func (s *Service) find() { s.store.Find() }", false},
		{"fourteen lines", "func (s *Service) Find() {\n" + strings.Repeat("\n", 11) + "s.store.Find()\n}", true},
		{"over fourteen lines", "func (s *Service) Find() {\n" + strings.Repeat("\n", 12) + "s.store.Find()\n}", false},
		{"two field calls", "func (s *Service) Find() { s.store.Find(); s.clock.Now() }", false},
		{"sister method only", "func (s *Service) Find() { s.helper() }", false},
		{"sister method plus field call", "func (s *Service) Find() { s.helper(); s.store.Find() }", true},
		{"nil guard", "func (s *Service) Find() { if s.store == nil { return }; s.store.Find() }", true},
		{"renamed method", "func (renamed *Service) Lookup() { renamed.store.Find() }", true},
		{"free function", "func Find() { s.store.Find() }", false},
		{"unnamed receiver", "func (*Service) Find() { s.store.Find() }", false},
		{"blank receiver", "func (_ *Service) Find() { s.store.Find() }", false},
		{"nested field", "func (s *Service) Find() { s.deps.clock.Now() }", true},
		{"closure", "func (s *Service) Find() { s.run(func() { s.store.Find() }) }", true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "fixture.go", "package fixture\n"+fixture.method, 0)
			if err != nil {
				t.Fatal(err)
			}
			method := file.Decls[0].(*ast.FuncDecl)
			if got := IsDelegation(method, fset); got != fixture.want {
				t.Fatalf("IsDelegation(%s) = %t, want %t", fixture.method, got, fixture.want)
			}
		})
	}
}

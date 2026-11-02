package querybudget

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

// ReadHistorical reads the pre-fragment Go map. Unsupported expressions fail
// closed, rather than guessing at a historical contract or executing source.
func ReadHistorical(source []byte) (map[string]Budget, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "query_budgets.go", source, 0)
	if err != nil {
		return nil, err
	}
	registry, err := historicalRegistry(file)
	if err != nil {
		return nil, err
	}

	budgets := map[string]Budget{}
	for _, element := range registry.Elts {
		budget, err := historicalEntry(element)
		if err != nil {
			return nil, err
		}
		if _, exists := budgets[budget.Scenario]; exists {
			return nil, fmt.Errorf("duplicate historical scenario %q", budget.Scenario)
		}
		budgets[budget.Scenario] = budget
	}
	return budgets, nil
}

func historicalRegistry(file *ast.File) (*ast.CompositeLit, error) {
	var registry *ast.CompositeLit
	for _, declaration := range file.Decls {
		if err := historicalDeclaration(&registry, declaration); err != nil {
			return nil, err
		}
	}

	if registry == nil || len(registry.Elts) == 0 {
		return nil, fmt.Errorf("missing or empty historical queryBudgets")
	}
	return registry, nil
}

func historicalDeclaration(registry **ast.CompositeLit, declaration ast.Decl) error {
	general, ok := declaration.(*ast.GenDecl)
	if !ok || general.Tok != token.VAR {
		return nil
	}
	for _, specification := range general.Specs {
		found, err := historicalDefinition(specification)
		if err != nil {
			return err
		}
		if found == nil {
			continue
		}
		if *registry != nil {
			return fmt.Errorf("ambiguous historical queryBudgets")
		}
		*registry = found
	}
	return nil
}

func historicalDefinition(specification ast.Spec) (*ast.CompositeLit, error) {
	value, ok := specification.(*ast.ValueSpec)
	if !ok {
		return nil, nil
	}
	for index, name := range value.Names {
		if name.Name != "queryBudgets" {
			continue
		}
		if index >= len(value.Values) {
			return nil, fmt.Errorf("missing historical queryBudgets initializer")
		}
		registry, ok := value.Values[index].(*ast.CompositeLit)
		if !ok {
			return nil, fmt.Errorf("historical queryBudgets is not a literal")
		}
		return registry, nil
	}
	return nil, nil
}

func historicalEntry(element ast.Expr) (Budget, error) {
	pair, ok := element.(*ast.KeyValueExpr)
	if !ok {
		return Budget{}, fmt.Errorf("historical query budget has no key")
	}
	key, ok := pair.Key.(*ast.BasicLit)
	if !ok || key.Kind != token.STRING {
		return Budget{}, fmt.Errorf("historical scenario is not a string literal")
	}
	scenario, err := strconv.Unquote(key.Value)
	if err != nil || scenario == "" {
		return Budget{}, fmt.Errorf("invalid historical scenario")
	}
	literal, ok := pair.Value.(*ast.CompositeLit)
	if !ok {
		return Budget{}, fmt.Errorf("historical budget %s is not a literal", scenario)
	}
	return historicalFields(Budget{Scenario: scenario}, literal)
}

func historicalFields(budget Budget, literal *ast.CompositeLit) (Budget, error) {
	seen := map[string]bool{}
	for _, element := range literal.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			return Budget{}, fmt.Errorf("historical budget %s requires keyed fields", budget.Scenario)
		}
		name, ok := field.Key.(*ast.Ident)
		if !ok || seen[name.Name] {
			return Budget{}, fmt.Errorf("invalid or duplicate historical field for %s", budget.Scenario)
		}
		seen[name.Name] = true
		if err := historicalField(&budget, name.Name, field.Value); err != nil {
			return Budget{}, err
		}
	}
	if !seen["max"] || budget.Max < 0 {
		return Budget{}, fmt.Errorf("invalid historical max for %s", budget.Scenario)
	}
	return budget, nil
}

func historicalField(budget *Budget, name string, value ast.Expr) error {
	switch name {
	case "max":
		literal, ok := value.(*ast.BasicLit)
		if !ok || literal.Kind != token.INT {
			return fmt.Errorf("historical max for %s is not an integer literal", budget.Scenario)
		}
		count, err := strconv.ParseInt(literal.Value, 0, 0)
		if err != nil {
			return err
		}
		budget.Max = int(count)
	case "exact":
		literal, ok := value.(*ast.Ident)
		if !ok || literal.Name != "true" && literal.Name != "false" {
			return fmt.Errorf("historical exact for %s is not boolean", budget.Scenario)
		}
		budget.Exact = literal.Name == "true"
	default:
		return fmt.Errorf("unknown historical query budget field %s", name)
	}
	return nil
}

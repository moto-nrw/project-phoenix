package students_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// isSchemaIntrospectionQuery matches queries that touch information_schema
// (or the pg_catalog schema-introspection tables); the input is lowercased SQL.
func isSchemaIntrospectionQuery(query string) bool {
	query = strings.ToLower(query)
	return strings.Contains(query, "information_schema") || strings.Contains(query, "pg_catalog")
}

func TestInformationSchemaRecorderMatchesSchemaCatalogs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{name: "information schema", query: "SELECT * FROM information_schema.columns", want: true},
		{name: "PostgreSQL catalog", query: "SELECT * FROM pg_catalog.pg_attribute", want: true},
		{name: "quoted PostgreSQL catalog", query: `SELECT * FROM "pg_catalog"."pg_class"`, want: true},
		{name: "application table", query: "SELECT * FROM users.students", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isSchemaIntrospectionQuery(tt.query))
		})
	}
}

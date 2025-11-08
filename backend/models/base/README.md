# Type-Safe Query System

This package provides a type-safe query system for the Project Phoenix backend.
It replaces the previously used untyped `map[string]interface{}` filter system
that was vulnerable to SQL injection.

## Overview

The new query system provides:

1. **Type-safe filters**: A structured way to build SQL conditions
2. **Parameterized queries**: All values are properly escaped via the Bun ORM
3. **Complex query support**: Compound conditions (AND/OR), sorting, pagination
4. **Easy to use API**: Fluent interface for building queries

## Usage Example

Here's a basic example of how to use the query system:

Example Go code:

```
package example

import (
    "context"
    "time"

    "github.com/moto-nrw/project-phoenix/models/base"
)

func exampleUsage() {
    // Create a filter
    filter := base.NewFilter().
        Equal("status", "active").
        GreaterThan("created_at", time.Now().AddDate(0, -1, 0)).
        In("category", "books", "electronics")

    // Add pagination
    options := base.NewQueryOptions().
        WithPagination(1, 20)
    options.Filter = filter

    // Add sorting
    sorting := base.NewSorting(
        base.SortField{Field: "created_at", Direction: base.SortDesc},
    )
    options.WithSorting(sorting)

    // Use with a repository
    ctx := context.Background()
    var repository Repository // Just for example
    entities, err := repository.List(ctx, options)
    if err != nil {
        // Handle error
    }
    // Use entities
}
```

## Available Filter Operations

The query system supports a wide range of filter operations:

- **Equality**: `Equal`, `NotEqual`
- **Comparison**: `GreaterThan`, `GreaterThanOrEqual`, `LessThan`,
  `LessThanOrEqual`
- **String matching**: `Like`, `ILike` (case-insensitive)
- **Null checking**: `IsNull`, `IsNotNull`
- **Collections**: `In`, `NotIn`
- **JSON**: `Contains`, `ContainedBy`, `HasKey`
- **Date ranges**: `DateRange`
- **Logical operators**: `And`, `Or`

## Migration Guide

To migrate existing repositories to use the new filter system:

1. Update repository implementations to accept the new `QueryOptions` type
2. Replace manual WHERE clause construction with filter conditions
3. Use the existing `repository_example.go` file as a reference

## Security Improvements

The new filter system eliminates SQL injection risks by:

1. Using parameterized queries throughout
2. Preventing direct string manipulation in WHERE clauses
3. Properly escaping all user-supplied values
4. Using Bun's query builder which handles quoting and escaping

## Recommendations

- Use this filter system for all database queries that accept user input
- Add more specific filter methods as needed for your domain
- Consider adding custom filter types for common query patterns

# Full Analysis of the Schema-Qualified Tables Fix

This solution was implemented in two phases:

1. **Initial Groundwork (Commit 2cf7c7b4)**: Set up the foundation for schema-qualified tables handling
2. **Final Fix**: Changed `TableExpr` to `ModelTableExpr` in the repository implementations

## 1. What Was the Problem?

The issue was related to how the Bun ORM was generating SQL for schema-qualified tables in PostgreSQL:

- When trying to register a user, the application returned a 201 Created response but no data was actually inserted in the database
- Looking at the SQL logs revealed the issue: `INSERT INTO "accounts" SELECT * FROM auth.accounts`
- Instead of properly inserting new values with a VALUES clause, it was generating a SELECT subquery that tried to copy data from an existing table
- This happened because `TableExpr("auth.accounts")` was being used instead of `ModelTableExpr("auth.accounts")` in the INSERT queries
- The SQL generated was effectively trying to clone the auth.accounts table, but since there were no rows to copy, it created an empty insertion

The technical difference:
- `TableExpr("auth.accounts")` - Creates a SQL expression for SELECT operations (SELECT * FROM auth.accounts)
- `ModelTableExpr("auth.accounts")` - Properly sets the target table name for INSERT operations

## 2. How We Fixed the Problem

### Phase 1: Initial Groundwork (Commit 2cf7c7b4)

First, we laid the foundation for correctly handling schema-qualified tables:

1. **Added Schema Information to Model Definition**:
   ```go
   // Added explicit schema info to struct tag
   type Account struct {
       base.Model    `bun:"schema:auth,table:accounts,alias:account"`
       // fields...
   }

   // Added TableName method
   func (a *Account) TableName() string {
       return "auth.accounts"
   }

   // Added query customization
   func (a *Account) BeforeAppendModel(query any) error {
       if q, ok := query.(*bun.SelectQuery); ok {
           q.TableExpr("auth.accounts AS account")
       }
       // Other query types...
   }
   ```

2. **Modified Repository Methods to Use Schema Qualifiers**:
   ```go
   // Changed from Model(account) to TableExpr
   err := r.db.NewSelect().
       TableExpr("auth.accounts").
       Where("LOWER(email) = LOWER(?)", email).
       Scan(ctx, account)
   ```

3. **Customized Account Creation Method**:
   ```go
   // Completely rewrote to use explicit schema
   query := r.db.NewInsert().
       Model(account).
       TableExpr("auth.accounts")
   ```

4. **Fixed Transaction Context Passing**:
   ```go
   // Store transaction in context
   ctx = context.WithValue(ctx, "tx", &tx)
   ```

5. **Set PostgreSQL Search Path**:
   ```go
   // Set the search_path to include auth schema
   _, err := db.Exec("SET search_path TO auth, public")
   ```

6. **Enabled SQL Debugging**:
   - Added `DB_DEBUG: "true"` environment variable
   - Added `--db_debug` command line flag

### Phase 2: Final Fix

Even with all the groundwork in place, there was still an issue. The SQL generated was still incorrect:

1. **Problem Identified**:
   - Using `TableExpr("auth.accounts")` in INSERT queries generated `INSERT INTO "accounts" SELECT * FROM auth.accounts`
   - This effectively tried to copy data from the auth.accounts table rather than inserting new values

2. **Repository Implementation Fix**:
   - Changed from `TableExpr` to the correct `ModelTableExpr` in INSERT queries:
   ```go
   query := r.db.NewInsert().
       Model(account).
       ModelTableExpr("auth.accounts") // CORRECT
   ```
   - Made this change in both regular and transaction-based query building

3. **Result**:
   - The SQL now properly includes the schema and VALUES clause:
   ```sql
   INSERT INTO auth.accounts ("id", ...) VALUES (DEFAULT, ...) RETURNING "id", ...
   ```
   - User registration now successfully stores data in the database.

## 3. How to Implement This for Other APIs

To apply this pattern to other repositories with schema-qualified tables:

1. **Consistent Model Struct Tags**:
   ```go
   type YourModel struct {
       base.Model     `bun:"schema:your_schema,table:your_table,alias:your_alias"`
       // fields...
   }
   ```

2. **Implement BeforeAppendModel in Models**:
   ```go
   func (m *YourModel) BeforeAppendModel(query any) error {
       if q, ok := query.(*bun.SelectQuery); ok {
           q.TableExpr("your_schema.your_table AS your_alias")
       }
       if q, ok := query.(*bun.InsertQuery); ok {
           q.ModelTableExpr("your_schema.your_table") // CRUCIAL for INSERT
       }
       if q, ok := query.(*bun.UpdateQuery); ok {
           q.TableExpr("your_schema.your_table")
       }
       if q, ok := query.(*bun.DeleteQuery); ok {
           q.TableExpr("your_schema.your_table")
       }
       return nil
   }
   ```

3. **Use ModelTableExpr in Repository Create Methods**:
   ```go
   func (r *YourRepository) Create(ctx context.Context, entity *your.Model) error {
       // Build query
       query := r.db.NewInsert().
           Model(entity).
           ModelTableExpr("your_schema.your_table") // NOT TableExpr

       // Handle transactions
       if tx, ok := ctx.Value("tx").(*bun.Tx); ok && tx != nil {
           query = tx.NewInsert().
               Model(entity).
               ModelTableExpr("your_schema.your_table") // NOT TableExpr
       }

       // Execute...
   }
   ```

4. **Ensure Proper Transaction Context**:
   - Pass transaction context explicitly in service layer methods:
   ```go
   ctx = context.WithValue(ctx, "tx", &tx)
   ```

5. **Database Search Path**:
   - As a fallback, set PostgreSQL search path to include schemas:
   ```go
   _, err := db.Exec("SET search_path TO your_schema, public")
   ```

6. **Enable SQL Debugging**:
   - For development, enable SQL logging to spot issues:
   ```go
   if viper.GetBool("db_debug") {
       db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
   }
   ```

By following these patterns consistently across your repositories, you'll ensure all schema-qualified table operations work correctly with Bun ORM, especially for INSERT operations.
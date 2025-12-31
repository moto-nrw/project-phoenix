---
description: Generate API documentation (routes.md + OpenAPI spec)
allowed-tools: Bash(cd:*), Bash(go run:*), Bash(docker compose:*)
---

# Generate API Documentation

Generate comprehensive API documentation from live router configuration.

## What This Generates

1. **`docs/routes.md`**: Complete route listing
   - All HTTP endpoints with methods (GET, POST, PUT, DELETE)
   - Middleware chains per route (auth, permissions, rate limiting)
   - Handler function mappings
   - Path parameter extraction

2. **`docs/openapi.yaml`**: OpenAPI 3.0.3 specification
   - Machine-readable API schema
   - Authentication schemes (Bearer JWT + API keys)
   - Path parameters automatically detected
   - Response schemas with status codes
   - Can be used with Swagger UI, Postman, API tools

## Execution

### Generate Both Files

```bash
cd backend && go run main.go gendoc
```

### Generate Only Routes

```bash
cd backend && go run main.go gendoc --routes
```

### Generate Only OpenAPI

```bash
cd backend && go run main.go gendoc --openapi
```

## Using Docker

If backend not running locally:

```bash
docker compose exec server ./main gendoc
docker compose exec server ./main gendoc --routes
docker compose exec server ./main gendoc --openapi
```

## When to Run

Run this command whenever:
- Adding new API endpoints
- Changing route paths
- Modifying middleware chains
- Updating permission requirements
- Before creating API documentation

## Generated Files Location

- **Routes**: `docs/routes.md` or `backend/docs/routes.md`
- **OpenAPI**: `docs/openapi.yaml` or `backend/docs/openapi.yaml`

## Using Generated Documentation

### Routes.md

Human-readable reference for:
- Available endpoints
- Required permissions per endpoint
- Handler function names (for code navigation)

Example output:
```
/api/groups
  - GET
    - [RequiresPermission(GroupsRead)]
    - [listGroups]
  - POST
    - [RequiresPermission(GroupsWrite)]
    - [createGroup]
```

### OpenAPI Spec

Machine-readable schema for:
- Swagger UI visualization
- Postman collection import
- Client SDK generation
- API testing tools

```yaml
paths:
  /api/groups:
    get:
      summary: "GET /api/groups"
      tags: ["Groups"]
      security:
        - bearerAuth: []
```

## Integration with Testing

Use generated documentation to:
1. Identify all endpoints for test coverage
2. Extract path parameters for test data
3. Verify permission requirements
4. Create Bruno test collection

## Before Deployment

Always regenerate documentation to ensure it matches code:

```bash
go run main.go gendoc
git add docs/
git commit -m "docs: update API documentation"
```

---
description: Run Bruno API tests with fresh authentication tokens
argument-hint: [domain|all]
allowed-tools: Bash(cd:*), Bash(bruno/*), Bash(./dev-test.sh:*)
---

# Run API Tests

Run Bruno API tests for Project Phoenix backend.

## Arguments

- `groups` - Test groups API (~44ms, 25 groups)
- `students` - Test students API (~50ms, 50 students)
- `rooms` - Test rooms API (~19ms, 24 rooms)
- `devices` - Test RFID device authentication (~117ms)
- `attendance` - Test attendance tracking (web + RFID)
- `all` - Run full test suite (~252ms, 52 tests)
- `examples` - View API usage examples
- `manual` - Run pre-release manual tests

If no argument provided, default to `all`.

## Execution

Navigate to bruno directory and run the test wrapper script:

```bash
cd bruno && ./dev-test.sh ${ARGUMENTS:-all}
```

The `dev-test.sh` script automatically:
1. Gets fresh admin token from backend
2. Runs Bruno tests with token authentication
3. Reports results with timing

## Expected Output

- Test summary (passed/failed)
- Execution time
- Any error messages

## Troubleshooting

If tests fail:
1. Check backend is running: `docker compose ps`
2. Verify test data exists: `docker compose exec server ./main seed`
3. Check backend logs: `docker compose logs -f server`

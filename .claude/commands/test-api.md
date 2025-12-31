---
description: Run Bruno API tests with consolidated test suite
argument-hint: [test-file|all]
allowed-tools: Bash(cd:*), Bash(bru:*), Bash(docker:*)
---

# Run API Tests

Run Bruno API tests for Project Phoenix backend using the consolidated test suite.

## Arguments

- `01-smoke` - Health checks (admin auth, groups API, device ping)
- `02-auth` - Authentication flows (admin, refresh, teacher, device)
- `03-resources` - Resource listings (groups, students, rooms, activities)
- `04-devices` - Device-specific endpoints
- `05-sessions` - Session lifecycle (10 test scenarios)
- `06-checkins` - Check-in/out flows (8 test scenarios)
- `07-attendance` - RFID + web attendance
- `08-rooms` - Room conflict validation
- `09-rfid` - RFID assignment operations
- `10-schulhof` - Schulhof auto-create workflow
- `11-claiming` - Group claiming workflow
- `all` - Run all 11 test files (~270ms)

If no argument provided, defaults to `all`.

## Execution

Navigate to bruno directory and run tests with Bruno CLI:

```bash
cd bruno

# Run requested tests (00-cleanup.bru runs automatically when using 0*.bru pattern)
if [ "${ARGUMENTS}" = "all" ] || [ -z "${ARGUMENTS}" ]; then
  bru run --env Local 0*.bru
else
  bru run --env Local ${ARGUMENTS}.bru
fi
```

The test suite automatically:
1. Authenticates using pre-request scripts (no external token management needed)
2. Runs hermetic tests (each file self-contained with setup and cleanup)
3. Reports results with timing and pass/fail status

## Expected Output

- Test execution status for each file
- Execution summary (requests passed/failed, duration)
- Console logs from test assertions

## Test Suite Structure

**Total**: 60 test scenarios across 12 files (includes automatic cleanup)
**Runtime**: ~340ms for complete suite
**Coverage**: Authentication, resources, sessions, check-ins, attendance, RFID, room conflicts, Schulhof workflow, group claiming

## Troubleshooting

**Tests consistently pass** thanks to automatic cleanup:
- 00-cleanup.bru runs first and ends all active sessions
- No manual intervention required

**Tests fail with authentication errors:**
- Check backend running: `docker compose ps`
- Verify seed data: `docker compose exec server ./main seed`

**Device auth failures:**
- Verify deviceApiKey in environments/Local.bru matches iot.devices table
- Ensure staffPIN is correct (default: 1234)

## Documentation

See `bruno/README.md` for complete test suite documentation.

#!/bin/bash
#
# check-hermetic-tests.sh - Verify tests follow hermetic testing patterns
#
# This script checks for common non-hermetic test patterns:
# 1. Hardcoded small integer IDs (int64(1), int64(2), etc.)
# 2. Test files with DB operations missing SetupTestDB
#
# Usage:
#   ./scripts/check-hermetic-tests.sh [--fix-suggestions]
#
# Exit codes:
#   0 - All checks passed
#   1 - Violations found

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BACKEND_DIR="$PROJECT_ROOT/backend"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

VIOLATIONS_FOUND=0
SHOW_FIX_SUGGESTIONS=false

if [[ "$1" == "--fix-suggestions" ]]; then
    SHOW_FIX_SUGGESTIONS=true
fi

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Hermetic Test Verification${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# -----------------------------------------------------------------------------
# Check 1: Hardcoded small integer IDs
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[Check 1]${NC} Scanning for hardcoded integer IDs..."
echo ""

# Pattern matches int64(1) through int64(9) which are likely hardcoded test IDs
# Excludes:
#   - Comments (lines starting with //)
#   - Loop variables and constants (i := int64(1))
#   - Legitimate small values in non-ID contexts
HARDCODED_ID_PATTERN='int64\([1-9]\)'

# Find violations, excluding known false positives
HARDCODED_VIOLATIONS=$(grep -rn "$HARDCODED_ID_PATTERN" "$BACKEND_DIR" \
    --include="*_test.go" \
    2>/dev/null | \
    grep -v "//.*int64(" | \
    grep -v "i := int64" | \
    grep -v "i = int64" | \
    grep -v "count.*int64" | \
    grep -v "offset.*int64" | \
    grep -v "limit.*int64" | \
    grep -v "page.*int64" | \
    grep -v "Weekday.*int64" | \
    grep -v "weekday.*int64" | \
    grep -v "day.*int64" | \
    grep -v "hour.*int64" | \
    grep -v "minute.*int64" | \
    grep -v "second.*int64" | \
    grep -v "duration.*int64" | \
    grep -v "timeout.*int64" | \
    grep -v "retry.*int64" | \
    grep -v "max.*int64" | \
    grep -v "min.*int64" | \
    grep -v "size.*int64" | \
    grep -v "len.*int64" | \
    grep -v "cap.*int64" | \
    grep -v "index.*int64" | \
    grep -v "999999" | \
    grep -v "GreaterOrEqual" | \
    grep -v "LessOrEqual" | \
    grep -v "Greater" | \
    grep -v "Less" | \
    grep -v "func()" | \
    grep -v "return &id" | \
    grep -v "_internal_test.go" | \
    grep -v "_mock_test.go" | \
    grep -v "models/" | \
    grep -v "invitation_service_test.go" || true)

if [[ -n "$HARDCODED_VIOLATIONS" ]]; then
    echo -e "${RED}Found potential hardcoded ID violations:${NC}"
    echo ""
    echo "$HARDCODED_VIOLATIONS" | while IFS= read -r line; do
        # Extract file path and line number
        FILE=$(echo "$line" | cut -d: -f1)
        LINE_NUM=$(echo "$line" | cut -d: -f2)
        CONTENT=$(echo "$line" | cut -d: -f3-)

        # Make path relative
        REL_PATH="${FILE#$PROJECT_ROOT/}"

        echo -e "  ${RED}!${NC} $REL_PATH:$LINE_NUM"
        echo -e "    ${CONTENT}"
        echo ""
    done

    if $SHOW_FIX_SUGGESTIONS; then
        echo -e "${YELLOW}Fix suggestion:${NC}"
        echo "  Instead of using hardcoded IDs like int64(1), use test fixtures:"
        echo ""
        echo "    // Before (non-hermetic):"
        echo "    result, err := repo.FindByID(ctx, int64(1))"
        echo ""
        echo "    // After (hermetic):"
        echo "    student := testpkg.CreateTestStudent(t, db, \"First\", \"Last\", \"1a\")"
        echo "    defer testpkg.CleanupTableRecords(t, db, \"users.students\", student.ID)"
        echo "    result, err := repo.FindByID(ctx, student.ID)"
        echo ""
    fi

    VIOLATIONS_FOUND=1
else
    echo -e "  ${GREEN}No hardcoded ID violations found.${NC}"
fi

echo ""

# -----------------------------------------------------------------------------
# Check 2: Test files with DB operations missing SetupTestDB
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[Check 2]${NC} Checking for test files missing SetupTestDB..."
echo ""

# Find test files that use bun.DB but don't use SetupTestDB
MISSING_SETUP_VIOLATIONS=""

for test_file in $(find "$BACKEND_DIR" -name "*_test.go" -type f 2>/dev/null); do
    # Skip the hermetic verification test itself
    if [[ "$test_file" == *"hermetic_verification_test.go"* ]]; then
        continue
    fi

    # Check if file imports bun or uses DB operations
    HAS_DB_OPS=false
    if grep -q "bun.DB\|\.NewSelect()\|\.NewInsert()\|\.NewUpdate()\|\.NewDelete()\|repositories\." "$test_file" 2>/dev/null; then
        HAS_DB_OPS=true
    fi

    # Check if file uses SetupTestDB or SetupAPITest
    USES_SETUP=false
    if grep -q "SetupTestDB\|setupTestDB\|SetupAPITest\|setupAPITest" "$test_file" 2>/dev/null; then
        USES_SETUP=true
    fi

    # Check if file uses mocks (legitimate alternative to real DB)
    USES_MOCKS=false
    if grep -q "sqlmock\|Mock\|mock\|Stub\|stub\|fake\|Fake" "$test_file" 2>/dev/null; then
        USES_MOCKS=true
    fi

    # Flag files with DB ops that don't use SetupTestDB and aren't mock-based
    if $HAS_DB_OPS && ! $USES_SETUP && ! $USES_MOCKS; then
        REL_PATH="${test_file#$PROJECT_ROOT/}"
        MISSING_SETUP_VIOLATIONS="${MISSING_SETUP_VIOLATIONS}${REL_PATH}\n"
    fi
done

if [[ -n "$MISSING_SETUP_VIOLATIONS" ]]; then
    echo -e "${RED}Found test files with DB operations missing SetupTestDB:${NC}"
    echo ""
    echo -e "$MISSING_SETUP_VIOLATIONS" | while IFS= read -r line; do
        if [[ -n "$line" ]]; then
            echo -e "  ${RED}!${NC} $line"
        fi
    done
    echo ""

    if $SHOW_FIX_SUGGESTIONS; then
        echo -e "${YELLOW}Fix suggestion:${NC}"
        echo "  Add SetupTestDB to initialize the test database:"
        echo ""
        echo "    func TestExample(t *testing.T) {"
        echo "        db := testpkg.SetupTestDB(t)"
        echo "        defer func() { _ = db.Close() }()"
        echo ""
        echo "        // ... test code using real database"
        echo "    }"
        echo ""
    fi

    VIOLATIONS_FOUND=1
else
    echo -e "  ${GREEN}All test files with DB operations use SetupTestDB.${NC}"
fi

echo ""

# -----------------------------------------------------------------------------
# Check 3: Tests using hardcoded email addresses that could conflict
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[Check 3]${NC} Checking for non-unique test email patterns..."
echo ""

# Find hardcoded emails without timestamp/unique suffix
HARDCODED_EMAIL_PATTERN='@test\.local"[^)]'
EMAIL_VIOLATIONS=$(grep -rn "$HARDCODED_EMAIL_PATTERN" "$BACKEND_DIR" \
    --include="*_test.go" \
    2>/dev/null | \
    grep -v "//.*@test\.local" | \
    grep -v "UnixNano" | \
    grep -v "time.Now" | \
    grep -v "unique" | \
    grep -v "Sprintf" | \
    grep -v "nonexistent" | \
    grep -v "invalid" | \
    grep -v "fake" || true)

if [[ -n "$EMAIL_VIOLATIONS" ]]; then
    echo -e "${YELLOW}Warning: Found potentially non-unique email addresses:${NC}"
    echo ""
    echo "$EMAIL_VIOLATIONS" | head -10 | while IFS= read -r line; do
        FILE=$(echo "$line" | cut -d: -f1)
        LINE_NUM=$(echo "$line" | cut -d: -f2)
        REL_PATH="${FILE#$PROJECT_ROOT/}"
        echo -e "  ${YELLOW}?${NC} $REL_PATH:$LINE_NUM"
    done
    echo ""
    echo -e "  ${YELLOW}Note:${NC} These may be intentional for error case testing."
    echo ""
else
    echo -e "  ${GREEN}No hardcoded email violations found.${NC}"
fi

echo ""

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

if [[ $VIOLATIONS_FOUND -eq 0 ]]; then
    echo -e "${GREEN}All hermetic test checks passed!${NC}"
    echo ""
    exit 0
else
    echo -e "${RED}Hermetic test violations found.${NC}"
    echo ""
    echo "Please fix the violations above to ensure tests are hermetic."
    echo "Hermetic tests should:"
    echo "  1. Create their own test data using fixtures"
    echo "  2. Never rely on hardcoded IDs that may not exist"
    echo "  3. Clean up after themselves"
    echo "  4. Be runnable in any order and in parallel"
    echo ""
    echo "Run with --fix-suggestions for detailed fix examples."
    echo ""
    exit 1
fi

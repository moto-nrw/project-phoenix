#!/bin/bash
# Cleanup script to checkout all students before running tests
# This ensures tests start with a clean state

echo "🧹 Cleaning up active visits before tests..."

# Get all active visits (students currently checked in)
ACTIVE_VISITS=$(docker compose exec -T postgres psql -U postgres -d postgres -t -c "
SELECT p.tag_id
FROM active.visits v
JOIN users.students s ON s.id = v.student_id
JOIN users.persons p ON p.id = s.person_id
WHERE v.exit_time IS NULL AND p.tag_id IS NOT NULL;
" 2>&1 | grep -v "^time=" | tr -d ' ')

if [ -z "$ACTIVE_VISITS" ]; then
  echo "✅ No active visits - database is clean"
  exit 0
fi

# Checkout each student
API_KEY="9YUQWdt4dLa013foUTRKdnaeEUPBsWj7"
PIN="1234"
CHECKED_OUT=0

echo "Found active visits, checking out students..."

for RFID in $ACTIVE_VISITS; do
  if [ -n "$RFID" ]; then
    echo "  Checking out student with RFID: $RFID"

    RESULT=$(curl -s -X POST http://localhost:8080/api/iot/checkin \
      -H "Authorization: Bearer $API_KEY" \
      -H "X-Staff-PIN: $PIN" \
      -H "Content-Type: application/json" \
      -d "{\"student_rfid\":\"$RFID\",\"action\":\"checkin\"}")

    STATUS=$(echo "$RESULT" | jq -r '.status' 2>/dev/null)

    if [ "$STATUS" = "success" ]; then
      CHECKED_OUT=$((CHECKED_OUT + 1))
      echo "    ✅ Checked out successfully"
    else
      ERROR=$(echo "$RESULT" | jq -r '.error' 2>/dev/null)
      echo "    ⚠️  Failed: $ERROR"
    fi
  fi
done

echo ""
echo "✅ Cleanup complete - checked out $CHECKED_OUT students"

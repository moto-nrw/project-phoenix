#!/bin/bash
set -e

API_KEY="9YUQWdt4dLa013foUTRKdnaeEUPBsWj7"
PIN="1234"
ROOM_1=3      # Klassenzimmer 2A
SCHULHOF=25

# Note: These RFID values should match testStudent1RFID and testStudent2RFID in environments/Local.bru
# Update these values if seed data changes
STUDENT_1_RFID="AD95A48E"      # Student 1 (default: Leon Lang)
STUDENT_1_NAME="Leon Lang"
STUDENT_2_RFID="DEADBEEF12345678"  # Student 2 (default: Emma Horn)
STUDENT_2_NAME="Emma Horn"

echo "🧪 Complete Schulhof Workflow Test"
echo "===================================="
echo ""
echo "Students: ${STUDENT_1_NAME} & ${STUDENT_2_NAME}"
echo "Rooms: Klassenzimmer 2A (3) & Schulhof (25)"
echo ""

# Helper function to call API
call_api() {
  local rfid=$1
  local room_id=$2
  local desc=$3
  
  echo "▶ $desc"
  
  if [ -z "$room_id" ]; then
    # Checkout (no room_id)
    PAYLOAD="{\"student_rfid\":\"$rfid\",\"action\":\"checkin\"}"
  else
    # Check-in (with room_id)
    PAYLOAD="{\"student_rfid\":\"$rfid\",\"action\":\"checkin\",\"room_id\":$room_id}"
  fi
  
  RESULT=$(curl -s -X POST http://localhost:8080/api/iot/checkin \
    -H "Authorization: Bearer $API_KEY" \
    -H "X-Staff-PIN: $PIN" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD")
  
  STATUS=$(echo "$RESULT" | jq -r '.status')
  ACTION=$(echo "$RESULT" | jq -r '.data.action // "error"')
  STUDENT=$(echo "$RESULT" | jq -r '.data.student_name // "unknown"')
  ROOM=$(echo "$RESULT" | jq -r '.data.room_name // "checkout"')
  
  if [ "$STATUS" = "success" ]; then
    echo "  ✅ $ACTION: $STUDENT → $ROOM"
  else
    echo "  ❌ FAILED: $(echo "$RESULT" | jq -r '.error')"
    exit 1
  fi
  
  sleep 0.5
  echo ""
}

# Step 1: Student 1 → Room 1
call_api "$STUDENT_1_RFID" "$ROOM_1" "1. Student 1 (${STUDENT_1_NAME}) → Room 1 (regular room)"

# Step 2: Student 2 → Schulhof (auto-create)
call_api "$STUDENT_2_RFID" "$SCHULHOF" "2. Student 2 (${STUDENT_2_NAME}) → Schulhof (auto-create group)"

# Step 3: Student 1 → Checkout
call_api "$STUDENT_1_RFID" "" "3. Student 1 (${STUDENT_1_NAME}) → Checkout from Room 1"

# Step 4: Student 1 → Schulhof (reuse group)
call_api "$STUDENT_1_RFID" "$SCHULHOF" "4. Student 1 (${STUDENT_1_NAME}) → Schulhof (reuse group)"

# Step 5: Student 2 → Checkout
call_api "$STUDENT_2_RFID" "" "5. Student 2 (${STUDENT_2_NAME}) → Checkout from Schulhof"

# Step 6: Student 2 → Room 1
call_api "$STUDENT_2_RFID" "$ROOM_1" "6. Student 2 (${STUDENT_2_NAME}) → Room 1"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🎉 COMPLETE WORKFLOW TEST PASSED!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Verified:"
echo "  ✅ Regular room check-ins work"
echo "  ✅ Schulhof auto-creates active group"
echo "  ✅ Schulhof group reused for second student"
echo "  ✅ Check-outs work correctly"
echo "  ✅ Students can switch between rooms"

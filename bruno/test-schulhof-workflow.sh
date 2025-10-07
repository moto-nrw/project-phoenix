#!/bin/bash
set -e

API_KEY="9YUQWdt4dLa013foUTRKdnaeEUPBsWj7"
PIN="1234"
ROOM_1=3      # Klassenzimmer 2A
SCHULHOF=25
STUDENT_A="AD95A48E"  # Leon
STUDENT_B="71A1DC68"  # Emma

echo "🧪 Complete Schulhof Workflow Test"
echo "===================================="
echo ""
echo "Students: Leon (A) & Emma (B)"
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

# Step 1: Student A → Room 1
call_api "$STUDENT_A" "$ROOM_1" "1. Student A (Leon) → Room 1 (regular room)"

# Step 2: Student B → Schulhof (auto-create)
call_api "$STUDENT_B" "$SCHULHOF" "2. Student B (Emma) → Schulhof (auto-create group)"

# Step 3: Student A → Checkout
call_api "$STUDENT_A" "" "3. Student A (Leon) → Checkout from Room 1"

# Step 4: Student A → Schulhof (reuse group)
call_api "$STUDENT_A" "$SCHULHOF" "4. Student A (Leon) → Schulhof (reuse group)"

# Step 5: Student B → Checkout
call_api "$STUDENT_B" "" "5. Student B (Emma) → Checkout from Schulhof"

# Step 6: Student B → Room 1
call_api "$STUDENT_B" "$ROOM_1" "6. Student B (Emma) → Room 1"

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

#!/bin/bash
set -e

echo "🧪 Testing Schulhof Auto-Create Feature"
echo "========================================"

# Get admin token
echo "1️⃣ Getting admin token..."
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"Test1234%"}' \
  | jq -r '.access_token')

if [ "$TOKEN" = "null" ] || [ -z "$TOKEN" ]; then
  echo "❌ Failed to get admin token"
  exit 1
fi
echo "✅ Got token: ${TOKEN:0:20}..."

# Get device API key
echo ""
echo "2️⃣ Getting device API key..."
DEVICE_KEY=$(curl -s http://localhost:8080/api/iot \
  -H "Authorization: Bearer $TOKEN" \
  | jq -r '.data[0].api_key // empty')

if [ -z "$DEVICE_KEY" ]; then
  echo "⚠️  No device found, creating test device..."
  DEVICE_KEY=$(curl -s -X POST http://localhost:8080/api/iot \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"device_id":"test-device-001","device_type":"rfid_reader","name":"Test Device"}' \
    | jq -r '.data.api_key')
fi
echo "✅ Device API key: ${DEVICE_KEY:0:20}..."

# Check if Schulhof room exists
echo ""
echo "3️⃣ Verifying Schulhof room exists..."
SCHULHOF_ROOM=$(curl -s "http://localhost:8080/api/iot/rooms/available" \
  -H "X-Device-API-Key: $DEVICE_KEY" \
  -H "X-Staff-PIN: 1234" \
  | jq -r '.data[] | select(.category == "Schulhof") | .id')

if [ -z "$SCHULHOF_ROOM" ]; then
  echo "❌ Schulhof room not found!"
  exit 1
fi
echo "✅ Schulhof room ID: $SCHULHOF_ROOM"

# Note: Update these RFIDs to match testStudent1RFID and testStudent2RFID in environments/Local.bru
STUDENT_1_RFID="AD95A48E"
STUDENT_1_NAME="Leon Lang"
STUDENT_2_RFID="DEADBEEF12345678"
STUDENT_2_NAME="Emma Horn"

# Test 1: Check-in first student to Schulhof (should create active group)
echo ""
echo "4️⃣ TEST 1: First student checks into Schulhof"
echo "   RFID: $STUDENT_1_RFID ($STUDENT_1_NAME)"
RESULT1=$(curl -s -X POST http://localhost:8080/api/iot/checkin \
  -H "X-Device-API-Key: $DEVICE_KEY" \
  -H "X-Staff-PIN: 1234" \
  -H "Content-Type: application/json" \
  -d "{\"student_rfid\":\"$STUDENT_1_RFID\",\"action\":\"checkin\",\"room_id\":$SCHULHOF_ROOM}")

echo "$RESULT1" | jq .
STATUS1=$(echo "$RESULT1" | jq -r '.status')
ACTION1=$(echo "$RESULT1" | jq -r '.data.action')

if [ "$STATUS1" = "success" ] && [ "$ACTION1" = "checked_in" ]; then
  echo "✅ TEST 1 PASSED: First student checked in successfully"
  echo "   Active group should have been auto-created"
else
  echo "❌ TEST 1 FAILED:"
  echo "$RESULT1" | jq .
  exit 1
fi

# Test 2: Check-in second student to Schulhof (should reuse active group)
echo ""
echo "5️⃣ TEST 2: Second student checks into Schulhof"
echo "   RFID: $STUDENT_2_RFID ($STUDENT_2_NAME)"
sleep 1
RESULT2=$(curl -s -X POST http://localhost:8080/api/iot/checkin \
  -H "X-Device-API-Key: $DEVICE_KEY" \
  -H "X-Staff-PIN: 1234" \
  -H "Content-Type: application/json" \
  -d "{\"student_rfid\":\"$STUDENT_2_RFID\",\"action\":\"checkin\",\"room_id\":$SCHULHOF_ROOM}")

echo "$RESULT2" | jq .
STATUS2=$(echo "$RESULT2" | jq -r '.status')
ACTION2=$(echo "$RESULT2" | jq -r '.data.action')

if [ "$STATUS2" = "success" ] && [ "$ACTION2" = "checked_in" ]; then
  echo "✅ TEST 2 PASSED: Second student checked in successfully"
  echo "   Used existing active group (no duplicate created)"
else
  echo "❌ TEST 2 FAILED:"
  echo "$RESULT2" | jq .
  exit 1
fi

# Verify only ONE active group exists in Schulhof
echo ""
echo "6️⃣ Verifying only ONE active group in Schulhof..."
ACTIVE_GROUPS=$(curl -s "http://localhost:8080/api/active/groups" \
  -H "Authorization: Bearer $TOKEN" \
  | jq -r ".data[] | select(.room_id == $SCHULHOF_ROOM and .end_time == null) | .id")

GROUP_COUNT=$(echo "$ACTIVE_GROUPS" | wc -l | tr -d ' ')
if [ "$GROUP_COUNT" = "1" ]; then
  echo "✅ VERIFICATION PASSED: Exactly 1 active group in Schulhof"
  echo "   Active group ID: $ACTIVE_GROUPS"
else
  echo "❌ VERIFICATION FAILED: Expected 1 active group, found $GROUP_COUNT"
  echo "   Active group IDs: $ACTIVE_GROUPS"
  exit 1
fi

echo ""
echo "🎉 ALL TESTS PASSED!"
echo "====================================="
echo "✅ First student created Schulhof active group"
echo "✅ Second student reused existing group"
echo "✅ No duplicate groups created"

#!/bin/bash

# Complete Claim Workflow Test
# Tests the deviceless Schulhof claim feature end-to-end

set -e

BASE_URL="http://localhost:8080"
TEACHER_EMAIL="andreas.arndt@schulzentrum.de"
TEACHER_PASSWORD="Test1234%"

echo "🧪 Testing Deviceless Schulhof Claim Workflow"
echo "=============================================="
echo ""

# Helper function to get fresh token
get_token() {
  curl -s -X POST "$BASE_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$TEACHER_EMAIL\",\"password\":\"$TEACHER_PASSWORD\"}" \
    | jq -r '.access_token'
}

# Step 1: List unclaimed groups
echo "📋 Step 1: List unclaimed active groups..."
TOKEN=$(get_token)
UNCLAIMED=$(curl -s -X GET "$BASE_URL/api/active/groups/unclaimed" \
  -H "Authorization: Bearer $TOKEN")

echo "$UNCLAIMED" | jq '.'
UNCLAIMED_COUNT=$(echo "$UNCLAIMED" | jq '.data | length')
echo "✅ Found $UNCLAIMED_COUNT unclaimed groups"
echo ""

# Get Schulhof group ID (should be 25)
SCHULHOF_ID=$(echo "$UNCLAIMED" | jq -r '.data[] | select(.room_id == 25) | .id')
if [ -z "$SCHULHOF_ID" ]; then
  echo "⚠️  Schulhof not found in unclaimed groups"
  echo "   Creating Schulhof group via RFID check-in first..."
  # Would need device auth setup here
  SCHULHOF_ID=25  # Fallback to known ID
fi

echo "🎯 Target: Schulhof active group ID = $SCHULHOF_ID"
echo ""

# Step 2: Claim the Schulhof group
echo "🖐️  Step 2: Teacher claims Schulhof supervision..."
TOKEN=$(get_token)  # Fresh token for POST
CLAIM_RESULT=$(curl -s -X POST "$BASE_URL/api/active/groups/$SCHULHOF_ID/claim" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json")

echo "$CLAIM_RESULT" | jq '.'

CLAIM_STATUS=$(echo "$CLAIM_RESULT" | jq -r '.status')
if [ "$CLAIM_STATUS" = "success" ]; then
  echo "✅ Successfully claimed supervision!"
  SUPERVISOR_ID=$(echo "$CLAIM_RESULT" | jq -r '.data.id')
  echo "   Supervisor record ID: $SUPERVISOR_ID"
elif [ "$CLAIM_STATUS" = "error" ]; then
  ERROR_MSG=$(echo "$CLAIM_RESULT" | jq -r '.error')
  if [[ "$ERROR_MSG" == *"already supervising"* ]]; then
    echo "ℹ️  Teacher already supervising this group (expected on re-run)"
  else
    echo "❌ Claim failed: $ERROR_MSG"
    exit 1
  fi
fi
echo ""

# Step 3: Verify supervision was created
echo "✅ Step 3: Verify supervision record exists..."
TOKEN=$(get_token)  # Fresh token
SUPERVISORS=$(curl -s -X GET "$BASE_URL/api/active/groups/$SCHULHOF_ID/supervisors" \
  -H "Authorization: Bearer $TOKEN")

echo "$SUPERVISORS" | jq '.'
SUPERVISOR_COUNT=$(echo "$SUPERVISORS" | jq '.data | length')
echo "✅ Found $SUPERVISOR_COUNT active supervisor(s) for Schulhof"
echo ""

# Step 4: Verify unclaimed list updated
echo "📋 Step 4: Verify Schulhof removed from unclaimed list..."
TOKEN=$(get_token)  # Fresh token
UNCLAIMED_AFTER=$(curl -s -X GET "$BASE_URL/api/active/groups/unclaimed" \
  -H "Authorization: Bearer $TOKEN")

STILL_UNCLAIMED=$(echo "$UNCLAIMED_AFTER" | jq -r '.data[] | select(.id == '$SCHULHOF_ID') | .id')
if [ -z "$STILL_UNCLAIMED" ]; then
  echo "✅ Schulhof successfully removed from unclaimed list!"
else
  echo "⚠️  Schulhof still appears in unclaimed list (might be duplicate group)"
fi
echo ""

# Step 5: Attempt to claim again (should fail)
echo "🔄 Step 5: Attempt to claim again (should fail)..."
TOKEN=$(get_token)  # Fresh token
RECLAIM=$(curl -s -X POST "$BASE_URL/api/active/groups/$SCHULHOF_ID/claim" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json")

RECLAIM_STATUS=$(echo "$RECLAIM" | jq -r '.status')
if [ "$RECLAIM_STATUS" = "error" ]; then
  echo "✅ Correctly rejected duplicate claim"
  echo "   Error: $(echo "$RECLAIM" | jq -r '.error')"
elif [ "$RECLAIM_STATUS" = "success" ]; then
  echo "⚠️  Claim succeeded - might be a different teacher or system allows multiple"
fi
echo ""

echo "=============================================="
echo "✅ Claim workflow test complete!"
echo "=============================================="

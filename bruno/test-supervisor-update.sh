#!/bin/bash

# Test script for supervisor update endpoint

BASE_URL="http://localhost:8080"
API_KEY="dev_a0b6e62275b5825ecf6e0e49d3eaa0c5cfa7522acf99b1a025068d0707d58429"
PIN="1234"

echo "=== Testing Supervisor Update Endpoint ==="

# Step 1: Start a session
echo "1. Starting session with initial supervisors [1, 2, 3]..."
START_RESPONSE=$(curl -s -X POST "$BASE_URL/api/iot/session/start" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Staff-PIN: $PIN" \
  -H "Content-Type: application/json" \
  -d '{
    "activity_id": 1,
    "room_id": 1,
    "supervisor_ids": [1, 2, 3],
    "force": true
  }')

# Extract active group ID
ACTIVE_GROUP_ID=$(echo "$START_RESPONSE" | grep -o '"active_group_id":[0-9]*' | cut -d: -f2)

if [ -z "$ACTIVE_GROUP_ID" ]; then
  echo "Failed to start session:"
  echo "$START_RESPONSE"
  exit 1
fi

echo "✓ Session started with ID: $ACTIVE_GROUP_ID"

# Step 2: Update supervisors
echo -e "\n2. Updating supervisors to [2, 4, 5]..."
UPDATE_RESPONSE=$(curl -s -X PUT "$BASE_URL/api/iot/session/$ACTIVE_GROUP_ID/supervisors" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Staff-PIN: $PIN" \
  -H "Content-Type: application/json" \
  -d '{
    "supervisor_ids": [2, 4, 5]
  }')

echo "Update response:"
echo "$UPDATE_RESPONSE" | jq '.'

# Step 3: Test edge cases
echo -e "\n3. Testing empty supervisor list (should fail)..."
EMPTY_RESPONSE=$(curl -s -X PUT "$BASE_URL/api/iot/session/$ACTIVE_GROUP_ID/supervisors" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Staff-PIN: $PIN" \
  -H "Content-Type: application/json" \
  -d '{
    "supervisor_ids": []
  }')

if [ -n "$EMPTY_RESPONSE" ]; then
  echo "$EMPTY_RESPONSE" | jq -r '.error // .message'
else
  echo "No response received"
fi

# Step 4: Test invalid supervisor ID
echo -e "\n4. Testing invalid supervisor ID (should fail)..."
INVALID_RESPONSE=$(curl -s -X PUT "$BASE_URL/api/iot/session/$ACTIVE_GROUP_ID/supervisors" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Staff-PIN: $PIN" \
  -H "Content-Type: application/json" \
  -d '{
    "supervisor_ids": [999999]
  }')

if [ -n "$INVALID_RESPONSE" ]; then
  echo "$INVALID_RESPONSE" | jq -r '.error // .message'
else
  echo "No response received"
fi

# Step 5: Test duplicate IDs (should deduplicate)
echo -e "\n5. Testing duplicate supervisor IDs [1, 1, 2, 2, 3]..."
DUP_RESPONSE=$(curl -s -X PUT "$BASE_URL/api/iot/session/$ACTIVE_GROUP_ID/supervisors" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Staff-PIN: $PIN" \
  -H "Content-Type: application/json" \
  -d '{
    "supervisor_ids": [1, 1, 2, 2, 3]
  }')

echo "$DUP_RESPONSE" | jq '.data.supervisors | length'
echo "Supervisors after deduplication:"
echo "$DUP_RESPONSE" | jq '.data.supervisors[].staff_id'

# Step 6: End session
echo -e "\n6. Ending session..."
END_RESPONSE=$(curl -s -X POST "$BASE_URL/api/iot/session/end" \
  -H "X-API-Key: $API_KEY" \
  -H "X-Device-PIN: $PIN")

echo "✓ Session ended"

echo -e "\n=== All tests completed ==="
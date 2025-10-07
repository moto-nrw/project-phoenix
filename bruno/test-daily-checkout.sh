#!/bin/bash

# Test Daily Checkout Feature
# This script tests the complete flow of checking a student into their home room
# and then checking them out to verify the daily checkout feature

echo "Testing Daily Checkout Feature"
echo "============================="
echo ""

# Get current time
CURRENT_HOUR=$(date +%H)
echo "Current time: $(date +%H:%M)"
echo "Daily checkout available after: 15:00"
echo ""

# First, we need to create an active session for the education group
# This simulates the teacher starting a session in the classroom
echo "Step 1: Starting education group session..."
echo "(Note: In real usage, this would be done via the webapp frontend)"
echo ""

# Step 2: Check in student to their home room
echo "Step 2: Checking in student Paula Vogel (RFID: 0717E589DBE0C0) to Room 101..."
./dev-test.sh device-daily-checkout-flow

echo ""
echo "Step 3: Checking out student from their home room..."
./dev-test.sh device-daily-checkout

echo ""
echo "Test complete!"
echo ""

if [ $CURRENT_HOUR -ge 15 ]; then
    echo "✅ Current time is after 15:00 - Daily checkout should be available"
    echo "   Expected action: 'checked_out_daily'"
else
    echo "⚠️  Current time is before 15:00 - Normal checkout only"
    echo "   Expected action: 'checked_out'"
fi
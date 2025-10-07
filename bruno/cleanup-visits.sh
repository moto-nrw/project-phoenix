#!/bin/bash
API_KEY="9YUQWdt4dLa013foUTRKdnaeEUPBsWj7"
PIN="1234"

echo "Cleaning up active visits..."

# Checkout both students
for RFID in "AD95A48E" "71A1DC68"; do
  curl -s -X POST http://localhost:8080/api/iot/checkin \
    -H "Authorization: Bearer $API_KEY" \
    -H "X-Staff-PIN: $PIN" \
    -H "Content-Type: application/json" \
    -d "{\"student_rfid\":\"$RFID\",\"action\":\"checkin\"}" > /dev/null
  echo "  Checked out student $RFID"
done

echo "✅ Cleanup complete"

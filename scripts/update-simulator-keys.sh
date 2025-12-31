#!/usr/bin/env bash
set -euo pipefail

SIM_CONFIG="backend/simulator/iot/simulator.yaml"

rows=$(docker compose exec -T postgres \
  psql -U postgres -d postgres \
       -At -F, \
       -c 'SELECT device_id, api_key FROM iot.devices ORDER BY device_id;')

if [[ -z "$rows" ]]; then
  echo "Failed to fetch API keys from database"
  exit 1
fi

tmp=$(mktemp)
cp "$SIM_CONFIG" "$tmp"

while IFS=, read -r device_id api_key; do
  if [[ -z "$device_id" || -z "$api_key" ]]; then
    continue
  fi
  echo "Updating $device_id -> $api_key"
  yq -i "
    (.devices[] | select(.device_id == \"$device_id\") | .api_key) = \"$api_key\"
  " "$tmp"
done <<< "$rows"

mv "$tmp" "$SIM_CONFIG"
echo "Simulator configuration updated: $SIM_CONFIG"

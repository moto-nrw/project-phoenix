#!/bin/bash
# Script to check certificate expiration dates and warn if certificates
# are nearing expiration (less than 30 days remaining)

CERT_DIR="certs"
WARN_DAYS=30

if [[ ! -d "$CERT_DIR" ]]; then
  echo "Error: Certificate directory not found at $CERT_DIR"
  exit 1
fi

check_expiration() {
  local cert=$1
  local cert_file="$CERT_DIR/$cert"
  
  if [[ ! -f "$cert_file" ]]; then
    echo "Warning: Certificate file $cert_file not found"
    return
  fi
  
  # Get certificate end date
  exp_date=$(openssl x509 -enddate -noout -in "$cert_file" | cut -d= -f2)
  
  # Convert to epoch seconds based on the platform (Linux or macOS)
  if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS date command
    exp_seconds=$(date -j -f "%b %d %H:%M:%S %Y %Z" "$exp_date" +%s 2>/dev/null)
  else
    # Linux date command
    exp_seconds=$(date -d "$exp_date" +%s 2>/dev/null)
  fi
  
  if [[ -z "$exp_seconds" ]]; then
    echo "Error: Could not parse expiration date for $cert"
    return
  fi
  
  # Get current date in seconds since epoch
  now_seconds=$(date +%s)
  
  # Calculate days until expiration
  seconds_diff=$((exp_seconds - now_seconds))
  days_left=$((seconds_diff / 86400))
  
  echo "$cert expires in $days_left days (on $exp_date)"
  
  if [[ $days_left -lt $WARN_DAYS ]]; then
    echo "WARNING: $cert will expire in less than $WARN_DAYS days!"
    echo "Please regenerate certificates using ./create-certs.sh"
  fi
}

echo "Checking certificate expiration dates..."
check_expiration "ca.crt"
check_expiration "server.crt"

echo "All certificates check complete."
exit 0
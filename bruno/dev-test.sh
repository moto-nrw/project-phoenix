#!/bin/bash

# Get auth token if needed
get_token() {
  curl -s -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" \
    -d '{"email":"admin@example.com","password":"Test1234%"}' \
    | jq -r '.access_token'
}

# Get teacher token for activities
get_teacher_token() {
  curl -s -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" \
    -d '{"email":"y.wenger@gmx.de","password":"Test1234%"}' \
    | jq -r '.access_token'
}

case $1 in
  "auth")     
    bru run dev/auth.bru --env Local ;;
  "groups")   
    TOKEN=$(get_token)
    bru run dev/groups.bru --env Local --env-var accessToken="$TOKEN" ;;
  "students") 
    TOKEN=$(get_token)
    bru run dev/students.bru --env Local --env-var accessToken="$TOKEN" ;;
  "rooms")    
    TOKEN=$(get_token)
    bru run dev/rooms.bru --env Local --env-var accessToken="$TOKEN" ;;
  "room-conflicts")
    TOKEN=$(get_token)
    echo "🧪 Testing Room Conflict Validation - Issue #3 Resolution..."
    bru run dev/room-conflict-final.bru --env Local --env-var accessToken="$TOKEN"
    echo ""
    echo "🧪 Testing again to verify conflict detection..."
    bru run dev/room-conflict-final.bru --env Local --env-var accessToken="$TOKEN" ;;
  "devices")  
    bru run dev/device-auth.bru --env Local ;;
  "sessions")
    bru run dev/device-session-conflict.bru --env Local
    bru run dev/device-session-current.bru --env Local
    bru run dev/device-session-management.bru --env Local
    bru run dev/device-session-end.bru --env Local ;;
  "checkin")
    bru run dev/device-checkin.bru --env Local
    bru run dev/device-checkout.bru --env Local
    bru run dev/device-checkin-errors.bru --env Local
    bru run dev/device-checkout-error.bru --env Local
    bru run dev/device-checkin-missing-room.bru --env Local ;;
  "activities")
    TEACHER_TOKEN=$(get_teacher_token)
    bru run dev/activities.bru --env Local --env-var teacherToken="$TEACHER_TOKEN"
    bru run dev/activity-create.bru --env Local --env-var teacherToken="$TEACHER_TOKEN" ;;
  "attendance-web")
    TOKEN=$(get_token)
    bru run dev/attendance-web.bru --env Local --env-var accessToken="$TOKEN" ;;
  "attendance-rfid")
    bru run dev/attendance.bru --env Local
    bru run dev/attendance-toggle.bru --env Local
    bru run dev/attendance-toggle-cancel.bru --env Local ;;
  "attendance")
    TOKEN=$(get_token)
    bru run dev/attendance-web.bru --env Local --env-var accessToken="$TOKEN"
    bru run dev/attendance.bru --env Local
    bru run dev/attendance-toggle.bru --env Local
    bru run dev/attendance-toggle-cancel.bru --env Local ;;
  "all")      
    bru run dev/ --env Local ;;
  "examples") 
    bru run examples/ --env Local ;;
  "manual")   
    TOKEN=$(get_token)
    bru run manual/ --env Local --env-var accessToken="$TOKEN" ;;
  *)          
    echo "Usage: ./dev-test.sh [auth|groups|students|rooms|room-conflicts|devices|sessions|checkin|activities|attendance-web|attendance-rfid|attendance|all|examples|manual]"
    echo ""
    echo "Examples:"
    echo "  ./dev-test.sh groups          # Test groups API (25 groups) - ~44ms"
    echo "  ./dev-test.sh students        # Test students API (50 students) - ~50ms"  
    echo "  ./dev-test.sh rooms           # Test rooms API (24 rooms) - ~19ms"
    echo "  ./dev-test.sh room-conflicts  # Test room conflict validation - Issue #3"
    echo "  ./dev-test.sh devices         # Test RFID device auth - ~117ms"
    echo "  ./dev-test.sh attendance-web  # Test web dashboard attendance endpoints"
    echo "  ./dev-test.sh attendance-rfid # Test RFID device attendance endpoints"
    echo "  ./dev-test.sh attendance      # Test both web and RFID attendance"
    echo "  ./dev-test.sh all             # Test everything - ~252ms"
    echo "  ./dev-test.sh examples        # View API examples"
    echo "  ./dev-test.sh manual          # Pre-release checks"
    echo ""
    exit 1 ;;
esac
#!/bin/bash

# Test teacher creation flow

echo "Testing teacher creation with backend role fixes"
echo "=============================================="

# First, let's check that the roles table is accessible
echo "1. Testing role query in postgres..."
docker compose exec postgres psql -U phoenix -d phoenix -c "SELECT * FROM auth.roles;"

# Test creating a teacher with a valid password (containing special character)
echo -e "\n2. Testing teacher creation API..."
TEACHER_DATA='{
  "first_name": "Test",
  "last_name": "Teacher",
  "specialization": "Mathematics",
  "generateCredentials": true,
  "credentialsData": {
    "email": "test.teacher@school.local",
    "username": "test_teacher",
    "password": "TestPass123!",
    "confirmPassword": "TestPass123!"
  }
}'

curl -X POST http://localhost:3000/api/teachers \
  -H "Content-Type: application/json" \
  -d "$TEACHER_DATA" \
  -v

echo -e "\n\nDone. Check the logs above for any errors."
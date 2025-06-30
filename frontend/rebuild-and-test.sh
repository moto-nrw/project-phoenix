#\!/bin/bash

echo "Rebuilding backend container and applying test data..."

# Rebuild the backend container
echo "1. Rebuilding backend container..."
docker compose build server

# Restart the backend service
echo "2. Restarting backend service..."
docker compose up -d server

# Wait for backend to be ready
echo "3. Waiting for backend to be ready..."
sleep 10

# Apply the test data
echo "4. Applying test data..."
docker exec -i project-phoenix-postgres-1 psql -U postgres < test-data.sql

echo "Done\! The backend has been rebuilt and test data applied."
echo ""
echo "You can now test with:"
echo "  - Student: Juna Günther (ID: 3) - in Gruppenraum Blau"
echo "  - Student: Jakob Günther (ID: 2) - in Gruppenraum Blau"
echo "  - Student: Pia Schäfer (ID: 12) - in Room 101"
echo "  - Student: Paula Hartmann (ID: 13) - in Room 102"
echo "  - Teacher: test@teachers.com - supervising multiple groups"

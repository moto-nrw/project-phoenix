@echo off
REM docker_grant_permissions.bat - Grants admin permissions using Docker
REM Usage: docker_grant_permissions.bat email@example.com

if "%1"=="" (
    echo Error: Please provide an email address
    echo Usage: docker_grant_permissions.bat email@example.com
    exit /b 1
)

set EMAIL=%1

echo Creating temporary SQL file...
copy add_admin_permissions.sql temp_permissions.sql

echo Replacing email in SQL file...
powershell -Command "(Get-Content temp_permissions.sql) -replace 'christan.kamann119@gmail.com', '%EMAIL%' | Set-Content temp_permissions.sql"

echo Granting permissions to %EMAIL% using Docker...
docker exec -i project-phoenix-postgres-1 psql -U postgres < temp_permissions.sql

echo Cleaning up...
del temp_permissions.sql

echo Done!
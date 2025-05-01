# Activity API Endpoints Overview

## Activity Group Categories

| Endpoint | Method | Description | Query Parameters |
|----------|--------|-------------|------------------|
| /categories | GET | List all activity group categories | - |
| /categories | POST | Create a new activity group category | - |
| /categories/{id} | GET | Get a specific activity group category | - |
| /categories/{id} | PUT | Update a specific activity group category | - |
| /categories/{id} | DELETE | Delete a specific activity group category | - |
| /categories/{id}/ags | GET | Get all activity groups in a category | - |

## Activity Groups

| Endpoint | Method | Description | Query Parameters |
|----------|--------|-------------|------------------|
| / | GET | List all activity groups | category_id, supervisor_id, is_open, active, search |
| / | POST | Create a new activity group | - |
| /{id} | GET | Get a specific activity group | - |
| /{id} | PUT | Update a specific activity group | - |
| /{id} | DELETE | Delete a specific activity group | - |
| /{id}/students | GET | Get all students enrolled in an activity group | - |

## Time Slots

| Endpoint | Method | Description | Request Body |
|----------|--------|-------------|-------------|
| /{id}/times | GET | Get all time slots for an activity group | - |
| /{id}/times | POST | Add a new time slot to an activity group | weekday, timespan_id |
| /{id}/times/{timeId} | DELETE | Delete a time slot from an activity group | - |

## Student Enrollment

| Endpoint | Method | Description | Path Parameters |
|----------|--------|-------------|----------------|
| /{id}/enroll/{studentId} | POST | Enroll a student in an activity group | id (ag_id), studentId |
| /{id}/enroll/{studentId} | DELETE | Unenroll a student from an activity group | id (ag_id), studentId |
| /student/{id}/ags | GET | Get all activity groups a student is enrolled in | id (student_id) |
| /student/available | GET | Get available activity groups for enrollment | student_id (query) |

## Public Access

| Endpoint | Method | Description | Query Parameters |
|----------|--------|-------------|------------------|
| /public | GET | Get a public list of active activity groups | category_id |
| /public/categories | GET | Get a public list of activity group categories | - |
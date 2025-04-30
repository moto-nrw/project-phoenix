# Student API Endpoints Overview

## Student Management

| Endpoint | Method | Description | Query Parameters |
|----------|--------|-------------|------------------|
| /students | GET | List all students | group_id, search, in_house, wc, school_yard |
| /students | POST | Create a new student | - |
| /students/{id} | GET | Get a specific student | - |
| /students/{id} | PUT | Update a specific student | - |
| /students/{id} | DELETE | Delete a specific student | - |

## Location Tracking

| Endpoint                       | Method | Description | Request Body |
|--------------------------------|--------|-------------|-------------|
| /students/update-location      | POST | Update a student's location flags | student_id, locations map |
| /students/register-in-room     | POST | Register a student in a room | student_id, device_id |
| /students/unregister-from-room | POST | Unregister a student from a room | student_id, device_id |

## Visit History

| Endpoint                             | Method | Description | Query Parameters |
|--------------------------------------|--------|-------------|------------------|
| /students/{id}/visits                | GET | Get visits for a specific student | date |
| /students/room/{id}/visits           | GET | Get visits for a specific room | date, active |
| /students/combined-group/{id}/visits | GET | Get visits for a combined group | date, active |

## Student Status

| Endpoint | Method | Description | Query Parameters |
|----------|--------|-------------|------------------|
| /students/{id}/status | GET | Get current status of a student | - |
| /students/public/summary | GET | Get summary of all students and their statuses | - |

## Feedback

| Endpoint                | Method | Description | Request Body |
|-------------------------|--------|-------------|-------------|
| /students/give-feedback | POST | Record feedback from a student | student_id, feedback_value, mensa_feedback |
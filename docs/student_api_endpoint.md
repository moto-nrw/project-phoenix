# Student API Endpoints Overview

## Student Management

| Endpoint | Method | Description | Query Parameters |
|----------|--------|-------------|------------------|
| /students | GET | List all students | group_id, search, in_house, wc, school_yard |
| /students | POST | Create a new student (requires existing custom_user_id) | - |
| /students/{id} | GET | Get a specific student | - |
| /students/{id} | PUT | Update a specific student | - |
| /students/{id} | DELETE | Delete a specific student | - |

## Combined User-Student Operations

| Endpoint | Method | Description | Query Parameters |
|----------|--------|-------------|------------------|
| /students/with-user | POST | Create a new student with a new user in one request | - |

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

## Detailed Documentation

### Create Student with User Endpoint

**Endpoint:** `/students/with-user`  
**Method:** `POST`  
**Description:** Creates a new CustomUser and Student in one transaction. This simplifies the process of creating a new student by handling the creation of both the user and student records in a single API call.

**Implementation Notes:**
- The endpoint creates a CustomUser first, then uses the generated ID to create the associated Student
- All operations occur in a single request for improved data consistency
- If user creation fails, the student will not be created
- The response includes the complete student record with all relationships
// lib/students-in-room-helpers.ts
//
// Types and mapper for GET /api/rooms/{id}/students-present.
//
// The backend already applies GDPR redaction server-side via
// gdpr.student_data_scope + DetermineStudentAccess, so any row the caller is
// not authorised to read full data for arrives with first_name / last_name /
// active_group_id / group_name / entry_time set to null on the wire. The
// student_id always survives so SSE pushes can diff rows even for the
// redacted set without leaking PII.
//
// Mapping rules:
//   - snake_case → camelCase
//   - int64 ids → strings (matches every other domain in the frontend)
//   - null on the wire → null in the model — never invent defaults; that
//     would silently un-redact a redacted row.

// Backend (snake_case, int64 numbers, ISO strings)
export interface BackendStudentInRoom {
  student_id: number;
  first_name: string | null;
  last_name: string | null;
  active_group_id: number | null;
  group_name: string | null;
  entry_time: string | null;
  has_full_access: boolean;
}

export interface BackendStudentsInRoomResponse {
  room_id: number;
  as_of: string;
  total_count: number;
  visible_count: number;
  students: BackendStudentInRoom[];
}

// Frontend (camelCase, string ids)
export interface StudentInRoom {
  studentId: string;
  firstName: string | null;
  lastName: string | null;
  activeGroupId: string | null;
  groupName: string | null;
  entryTime: string | null;
  hasFullAccess: boolean;
}

export interface StudentsInRoomResponse {
  roomId: string;
  asOf: string;
  totalCount: number;
  visibleCount: number;
  students: StudentInRoom[];
}

export function mapStudentInRoom(row: BackendStudentInRoom): StudentInRoom {
  return {
    studentId: row.student_id.toString(),
    firstName: row.first_name,
    lastName: row.last_name,
    activeGroupId:
      row.active_group_id === null ? null : row.active_group_id.toString(),
    groupName: row.group_name,
    entryTime: row.entry_time,
    hasFullAccess: row.has_full_access,
  };
}

export function mapStudentsInRoomResponse(
  payload: BackendStudentsInRoomResponse,
): StudentsInRoomResponse {
  return {
    roomId: payload.room_id.toString(),
    asOf: payload.as_of,
    totalCount: payload.total_count,
    visibleCount: payload.visible_count,
    students: payload.students.map(mapStudentInRoom),
  };
}

/**
 * Staff-side client for reading the newest parent notes left for a
 * student via the parents portal. Read-only — staff cannot author or
 * delete guardian notes. Mirrors api/students.ParentNoteResponse (ids
 * already stringified by the backend).
 */

export interface StudentParentNote {
  id: string;
  student_id: string;
  body: string;
  created_at: string;
}

interface ApiResponse<T> {
  status?: string;
  data?: T;
  error?: string;
}

export async function fetchStudentParentNotes(
  studentId: string,
): Promise<StudentParentNote[]> {
  const response = await fetch(`/api/students/${studentId}/parent-notes`);
  if (!response.ok) {
    throw new Error("Elternnachrichten konnten nicht geladen werden");
  }
  const result = (await response.json()) as ApiResponse<StudentParentNote[]>;
  return result.data ?? [];
}

export type StudentStatusKind = "sick" | "excused";

export interface StudentStatusDay {
  id: string;
  student_id: string;
  date: string;
  status: StudentStatusKind;
  label: string;
  reported_at: string;
  cleared_at?: string | null;
  source: string;
  created_at: string;
  updated_at: string;
}

interface ApiResponse<T> {
  status: string;
  data?: T;
  message?: string;
  error?: string;
}

interface BackendStudentStatusDay {
  id: number;
  student_id: number;
  date: string;
  status: StudentStatusKind;
  label: string;
  reported_at: string;
  cleared_at?: string | null;
  source: string;
  created_at: string;
  updated_at: string;
}

function mapStatusDay(row: BackendStudentStatusDay): StudentStatusDay {
  return {
    ...row,
    id: row.id.toString(),
    student_id: row.student_id.toString(),
  };
}

async function parseApiResult<T>(
  response: Response,
  fallback: string,
): Promise<T> {
  const result = (await response.json()) as ApiResponse<T>;
  if (result.status === "error" || !result.data) {
    throw new Error(result.error ?? fallback);
  }
  return result.data;
}

export async function fetchStudentStatusDays(
  studentId: string,
  from: string,
  to: string,
): Promise<StudentStatusDay[]> {
  const response = await fetch(
    `/api/students/${studentId}/status-days?from=${from}&to=${to}`,
  );
  if (!response.ok) {
    throw new Error("Geplante Einträge konnten nicht geladen werden");
  }
  const data = await parseApiResult<BackendStudentStatusDay[]>(
    response,
    "Geplante Einträge konnten nicht geladen werden",
  );
  return data.map(mapStatusDay);
}

export async function createStudentStatusDays(
  studentId: string,
  status: StudentStatusKind,
  dates: string[],
): Promise<StudentStatusDay[]> {
  const response = await fetch(`/api/students/${studentId}/status-days`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ status, dates }),
  });
  if (!response.ok) {
    throw new Error("Geplante Einträge konnten nicht gespeichert werden");
  }
  const data = await parseApiResult<BackendStudentStatusDay[]>(
    response,
    "Geplante Einträge konnten nicht gespeichert werden",
  );
  return data.map(mapStatusDay);
}

export async function deleteStudentStatusDay(
  studentId: string,
  statusDayId: string,
): Promise<void> {
  const response = await fetch(
    `/api/students/${studentId}/status-days/${statusDayId}`,
    { method: "DELETE" },
  );
  if (!response.ok) {
    throw new Error("Geplanter Eintrag konnte nicht gelöscht werden");
  }
}

export interface StudentPartialAbsence {
  id: string;
  studentId: string;
  date: string;
  fromTime: string;
  reason?: string;
  pickupTime?: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

interface ApiResponse<T> {
  status: string;
  data?: T;
  error?: string;
}

interface BackendPartialAbsence {
  id: number;
  student_id: number;
  date: string;
  from_time: string;
  reason?: string;
  pickup_time?: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

function mapPartialAbsence(row: BackendPartialAbsence): StudentPartialAbsence {
  return {
    id: row.id.toString(),
    studentId: row.student_id.toString(),
    date: row.date,
    fromTime: row.from_time,
    reason: row.reason,
    pickupTime: row.pickup_time,
    createdBy: row.created_by.toString(),
    createdAt: row.created_at,
    updatedAt: row.updated_at,
  };
}

async function parseData<T>(response: Response, fallback: string): Promise<T> {
  const body = (await response.json()) as ApiResponse<T>;
  if (!response.ok || body.status === "error" || body.data === undefined) {
    throw new Error(body.error ?? fallback);
  }
  return body.data;
}

export async function fetchStudentPartialAbsences(
  studentId: string,
  from: string,
  to: string,
): Promise<StudentPartialAbsence[]> {
  const response = await fetch(
    `/api/students/${studentId}/partial-absences?from=${from}&to=${to}`,
  );
  const rows = await parseData<BackendPartialAbsence[]>(
    response,
    "Teilentschuldigungen konnten nicht geladen werden",
  );
  return rows.map(mapPartialAbsence);
}

export async function saveStudentPartialAbsence(
  studentId: string,
  partialAbsenceId: string | null,
  date: string,
  fromTime: string,
  reason?: string,
): Promise<StudentPartialAbsence> {
  const response = await fetch(
    partialAbsenceId
      ? `/api/students/${studentId}/partial-absences/${partialAbsenceId}`
      : `/api/students/${studentId}/partial-absences`,
    {
      method: partialAbsenceId ? "PUT" : "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(
        reason
          ? { date, from_time: fromTime, reason }
          : { date, from_time: fromTime },
      ),
    },
  );
  const row = await parseData<BackendPartialAbsence>(
    response,
    "Teilentschuldigung konnte nicht gespeichert werden",
  );
  return mapPartialAbsence(row);
}

export async function deleteStudentPartialAbsence(
  studentId: string,
  partialAbsenceId: string,
): Promise<void> {
  const response = await fetch(
    `/api/students/${studentId}/partial-absences/${partialAbsenceId}`,
    { method: "DELETE" },
  );
  if (!response.ok) {
    throw new Error("Teilentschuldigung konnte nicht entfernt werden");
  }
}

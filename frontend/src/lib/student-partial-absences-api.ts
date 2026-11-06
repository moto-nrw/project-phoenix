import { bffJson } from "~/lib/bff-fetch";

export interface StudentPartialAbsence {
  id: string;
  studentId: string;
  date: string;
  fromTime: string;
  reason?: string;
  pickupTime?: string;
  /** true = automatisch aus vorverlegter Abholzeit abgeleitet (#2360). */
  auto: boolean;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

interface BackendPartialAbsence {
  id: number;
  student_id: number;
  date: string;
  from_time: string;
  reason?: string;
  pickup_time?: string;
  auto?: boolean;
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
    auto: row.auto ?? false,
    createdBy: row.created_by.toString(),
    createdAt: row.created_at,
    updatedAt: row.updated_at,
  };
}

function partialAbsencesPath(studentId: string, id?: string | null): string {
  const base = `/api/students/${encodeURIComponent(studentId)}/partial-absences`;
  return id ? `${base}/${encodeURIComponent(id)}` : base;
}

export async function fetchStudentPartialAbsences(
  studentId: string,
  from: string,
  to: string,
): Promise<StudentPartialAbsence[]> {
  const rows = await bffJson<BackendPartialAbsence[]>(
    `${partialAbsencesPath(studentId)}?from=${from}&to=${to}`,
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
  const row = await bffJson<BackendPartialAbsence>(
    partialAbsencesPath(studentId, partialAbsenceId),
    {
      method: partialAbsenceId ? "PUT" : "POST",
      body: reason
        ? { date, from_time: fromTime, reason }
        : { date, from_time: fromTime },
    },
  );
  return mapPartialAbsence(row);
}

export async function deleteStudentPartialAbsence(
  studentId: string,
  partialAbsenceId: string,
): Promise<void> {
  await bffJson<unknown>(partialAbsencesPath(studentId, partialAbsenceId), {
    method: "DELETE",
  });
}

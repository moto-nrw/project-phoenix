import { getSession } from "next-auth/react";

export interface ArrivalSchedule {
  id: number;
  student_id: number;
  weekday: number;
  weekday_name: string;
  expected_arrival: string;
  notes?: string | null;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface ArrivalException {
  id: number;
  student_id: number;
  exception_date: string;
  expected_arrival?: string | null;
  reason?: string | null;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface ArrivalNote {
  id: number;
  student_id: number;
  note_date: string;
  content: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface ArrivalData {
  schedules: ArrivalSchedule[];
  exceptions: ArrivalException[];
  notes: ArrivalNote[];
}

export interface ArrivalScheduleInput {
  weekday: number;
  expected_arrival: string;
  notes?: string | null;
}

interface UpdateArrivalSchedulesBody {
  schedules: ArrivalScheduleInput[];
}

interface BulkArrivalRequestBody {
  school_class: string;
  schedules: ArrivalScheduleInput[];
}

interface ApiWrapper<T> {
  data: T;
}

export const WEEKDAYS: { value: number; label: string; short: string }[] = [
  { value: 1, label: "Montag", short: "Mo" },
  { value: 2, label: "Dienstag", short: "Di" },
  { value: 3, label: "Mittwoch", short: "Mi" },
  { value: 4, label: "Donnerstag", short: "Do" },
  { value: 5, label: "Freitag", short: "Fr" },
];

function unwrap<T>(value: ApiWrapper<T> | T): T {
  if (
    value &&
    typeof value === "object" &&
    "data" in (value as ApiWrapper<T>)
  ) {
    return (value as ApiWrapper<T>).data;
  }
  return value as T;
}

async function authHeaders(): Promise<HeadersInit> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  const session = await getSession();
  if (session?.user?.token) {
    headers.Authorization = `Bearer ${session.user.token}`;
  }
  return headers;
}

async function parseResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const text = await response.text().catch(() => "");
    throw new Error(
      text
        ? `Request failed (${response.status}): ${text}`
        : `Request failed (${response.status})`,
    );
  }
  if (response.status === 204) {
    return undefined as T;
  }
  const payload = (await response.json()) as ApiWrapper<T> | T;
  return unwrap<T>(payload);
}

export async function fetchArrivalData(
  studentId: string,
): Promise<ArrivalData> {
  const response = await fetch(`/api/students/${studentId}/arrival-schedules`, {
    method: "GET",
    headers: await authHeaders(),
    credentials: "include",
  });
  return parseResponse<ArrivalData>(response);
}

export async function updateArrivalSchedules(
  studentId: string,
  schedules: ArrivalScheduleInput[],
): Promise<ArrivalSchedule[]> {
  const body: UpdateArrivalSchedulesBody = { schedules };
  const response = await fetch(`/api/students/${studentId}/arrival-schedules`, {
    method: "PUT",
    headers: await authHeaders(),
    credentials: "include",
    body: JSON.stringify(body),
  });
  return parseResponse<ArrivalSchedule[]>(response);
}

export async function bulkUpsertArrivalByClass(
  schoolClass: string,
  schedules: ArrivalScheduleInput[],
): Promise<unknown> {
  const body: BulkArrivalRequestBody = {
    school_class: schoolClass,
    schedules,
  };
  const response = await fetch("/api/students/arrival-schedules/bulk", {
    method: "POST",
    headers: await authHeaders(),
    credentials: "include",
    body: JSON.stringify(body),
  });
  return parseResponse<unknown>(response);
}

// Client-side API for Klassenlisteneinträge (#2382): Kinder des
// Klassenverbands OHNE OGS-Datensatz — nur Name + Klasse. Die Einträge
// erscheinen ausschließlich auf Klassenlisten und in der Klassenansicht
// ("Keine Betreuung"); Anwesenheit, Betreuungsplanung und Elternportal
// kennen sie strukturell nicht.

export interface ClassListEntry {
  id: string;
  firstName: string;
  lastName: string;
  schoolClass: string;
  createdAt: string;
  // IDs regulärer Kinder mit gleichem Namen in derselben Klasse — der
  // Hinweis für die bewusste Zuordnung. Nie automatisch zusammengeführt.
  matchingStudentIds: string[];
}

export interface ClassListEntryInput {
  firstName: string;
  lastName: string;
  schoolClass: string;
}

interface WireEntry {
  id: number;
  first_name: string;
  last_name: string;
  school_class: string;
  created_at: string;
  matching_student_ids?: number[];
}

interface Envelope<T> {
  data: T;
  error?: string;
  message?: string;
}

function mapEntry(wire: WireEntry): ClassListEntry {
  return {
    id: wire.id.toString(),
    firstName: wire.first_name,
    lastName: wire.last_name,
    schoolClass: wire.school_class,
    createdAt: wire.created_at,
    matchingStudentIds: (wire.matching_student_ids ?? []).map((id) =>
      id.toString(),
    ),
  };
}

// Eigener Fetch statt authFetch: die Backend-Fehlermeldungen (z.B. "existiert
// in dieser Klasse bereits") sind deutschsprachige Nutzertexte und müssen die
// aufrufende UI erreichen — authFetch wirft nur den generischen Statustext.
async function request<T>(
  url: string,
  method: "GET" | "POST" | "PUT" | "DELETE",
  body?: unknown,
): Promise<T> {
  const response = await fetch(url, {
    method,
    credentials: "include",
    cache: "no-store",
    ...(body !== undefined && {
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }),
  });
  const payload = (await response.json().catch(() => null)) as
    (Envelope<unknown> & Record<string, unknown>) | null;
  if (!response.ok) {
    const message =
      (typeof payload?.error === "string" && payload.error) ||
      (typeof payload?.message === "string" && payload.message) ||
      `API error (${response.status})`;
    throw new Error(message);
  }
  return payload as T;
}

function toWireInput(input: ClassListEntryInput) {
  return {
    first_name: input.firstName.trim(),
    last_name: input.lastName.trim(),
    school_class: input.schoolClass.trim(),
  };
}

export async function fetchClassListEntries(): Promise<ClassListEntry[]> {
  const envelope = await request<Envelope<WireEntry[]>>(
    "/api/class-list-entries",
    "GET",
  );
  return (envelope.data ?? []).map(mapEntry);
}

export async function createClassListEntry(
  input: ClassListEntryInput,
): Promise<ClassListEntry> {
  const envelope = await request<Envelope<WireEntry>>(
    "/api/class-list-entries",
    "POST",
    toWireInput(input),
  );
  return mapEntry(envelope.data);
}

export async function updateClassListEntry(
  id: string,
  input: ClassListEntryInput,
): Promise<ClassListEntry> {
  const envelope = await request<Envelope<WireEntry>>(
    `/api/class-list-entries/${id}`,
    "PUT",
    toWireInput(input),
  );
  return mapEntry(envelope.data);
}

export async function deleteClassListEntry(id: string): Promise<void> {
  await request<Envelope<null>>(`/api/class-list-entries/${id}`, "DELETE");
}

export async function assignClassListEntry(
  id: string,
  studentId: string,
): Promise<void> {
  await request<Envelope<null>>(
    `/api/class-list-entries/${id}/assign`,
    "POST",
    // int64-IDs reisen als JSON-String (Backend bindet mit `,string`): als
    // JSON-Zahl würden IDs oberhalb von 2^53 beim Parsen gerundet.
    {
      student_id: studentId,
    },
  );
}

/** Minimal, read-only identity. Never a detail record or an access grant. */
export interface ListPreview {
  readonly id: string;
  readonly name: string;
  readonly context?: string;
}

function object(value: unknown): Record<string, unknown> | undefined {
  return value !== null && typeof value === "object"
    ? (value as Record<string, unknown>)
    : undefined;
}

export function previewRow(data: unknown, id: string) {
  const record = object(data);
  const rows = Array.isArray(data)
    ? data
    : [record?.students, record?.entries, record?.course_students]
        .filter(Array.isArray)
        .flat();
  if (!Array.isArray(rows)) return undefined;
  for (const value of rows) {
    const row = object(value);
    if ((row?.student_id ?? row?.id) === id) return row;
  }
  return undefined;
}

export function previewText(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value : undefined;
}

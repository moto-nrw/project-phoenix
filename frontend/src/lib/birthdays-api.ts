import { fetchWithAuth } from "./fetch-with-auth";
import { trackEvent } from "./analytics";

/**
 * Birthday display + staff birthday list (#1542).
 *
 * The backend already applies the school settings and the personal opt-outs,
 * so everything returned here is safe to render as-is — the frontend never
 * re-decides who may be shown.
 */

export type BirthdayKind = "student" | "staff";

export interface BirthdayCelebration {
  kind: BirthdayKind;
  id: string;
  name: string;
  groupName?: string;
  schoolClass?: string;
  /** The day the birthday is SHOWN on ("YYYY-MM-DD"), not always the birth date. */
  date: string;
  /** Completed years reached. Children only — a staff entry never carries one. */
  age?: number;
  isToday: boolean;
}

export interface BirthdayOverview {
  enabled: boolean;
  includeStaff: boolean;
  today: string;
  celebrations: BirthdayCelebration[];
}

interface BackendCelebration {
  kind: BirthdayKind;
  id: number;
  name: string;
  group_name?: string;
  school_class?: string;
  date: string;
  age?: number;
  is_today: boolean;
}

interface BackendOverview {
  enabled: boolean;
  include_staff: boolean;
  today: string;
  celebrations: BackendCelebration[] | null;
}

export function mapBirthdayOverview(
  response: BackendOverview,
): BirthdayOverview {
  return {
    enabled: response.enabled,
    includeStaff: response.include_staff,
    today: response.today,
    celebrations: (response.celebrations ?? []).map((celebration) => ({
      kind: celebration.kind,
      id: celebration.id.toString(),
      name: celebration.name,
      groupName: celebration.group_name,
      schoolClass: celebration.school_class,
      date: celebration.date,
      age: celebration.age,
      isToday: celebration.is_today,
    })),
  };
}

/** Client-side fetcher for SWR — calls the Next.js BFF route. */
export async function fetchBirthdayOverviewClient(): Promise<BirthdayOverview> {
  const response = await fetchWithAuth("/api/birthdays");

  if (!response.ok) {
    throw new Error(`Birthday fetch failed: ${response.status}`);
  }

  const json = (await response.json()) as { data: BackendOverview };
  return mapBirthdayOverview(json.data);
}

export async function fetchBirthdayOptOut(): Promise<boolean> {
  const response = await fetchWithAuth("/api/birthdays/opt-out");

  if (!response.ok) {
    throw new Error(`Birthday opt-out fetch failed: ${response.status}`);
  }

  const json = (await response.json()) as { data: { opt_out: boolean } };
  return json.data.opt_out;
}

export async function updateBirthdayOptOut(optOut: boolean): Promise<boolean> {
  const response = await fetchWithAuth("/api/birthdays/opt-out", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ opt_out: optOut }),
  });

  if (!response.ok) {
    throw new Error(`Birthday opt-out update failed: ${response.status}`);
  }

  const json = (await response.json()) as { data: { opt_out: boolean } };
  return json.data.opt_out;
}

export type StaffBirthdayExportFormat = "pdf" | "docx" | "xlsx";

export interface StaffBirthdayExportRequest {
  format: StaffBirthdayExportFormat;
  title?: string;
  /** Birth months as "01".."12"; empty means the whole year. */
  months?: string[];
}

export async function exportStaffBirthdays(
  request: StaffBirthdayExportRequest,
): Promise<void> {
  const response = await fetch("/api/birthdays/staff-export", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });

  if (!response.ok) {
    throw new Error(await readExportError(response));
  }

  trackEvent("data_exported", {
    export_type: "staff_birthdays",
    format: request.format,
  });

  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download =
    filenameFromDisposition(response) ??
    `geburtstagsliste-personal.${request.format}`;
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

// The export proxy returns errors as {"error":"..."}; a plain response may send
// a bare text body. Prefer the JSON message so the toast never shows an empty
// string or a raw envelope.
async function readExportError(response: Response): Promise<string> {
  const text = await response.text();
  if (!text) return "Export fehlgeschlagen";
  try {
    const parsed = JSON.parse(text) as { error?: unknown };
    if (typeof parsed.error === "string" && parsed.error.trim()) {
      return parsed.error;
    }
  } catch {
    // Not JSON — forward the raw text.
  }
  return text;
}

function filenameFromDisposition(response: Response): string | null {
  const disposition = response.headers.get("content-disposition");
  if (!disposition) return null;
  const match = /filename="?([^";]+)"?/i.exec(disposition);
  return match?.[1] ?? null;
}

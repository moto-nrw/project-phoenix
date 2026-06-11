// Client API + types for slot lists (issue #1565): lists from selected
// timetable slots or Ganztag pickup cohorts with three data modes
// (Plan / Ist / Abgleich) and PDF/XLSX export.

export type SlotListTarget = "slots" | "pickup_cohort";

export type SlotListPickupCohort = "short_day" | "long_day";

export type SlotListSource = "planned" | "actual" | "reconciliation";

export type SlotListGroupBy = "" | "slot" | "room" | "class" | "pickup_time";

export type SlotListFormat = "pdf" | "xlsx";

export interface SlotListRequest {
  date: string; // YYYY-MM-DD
  target: SlotListTarget;
  pickup_cohort?: SlotListPickupCohort;
  source: SlotListSource;
  /** Selected timetable slots. Omit = all selectable slots; [] = none selected. */
  instance_ids?: number[];
  /** Optional education-group filter. Omit/empty = all groups. */
  group_ids?: number[];
  /** Optional school-class filter. Omit/empty = all classes. */
  classes?: string[];
  /** Section the preview/export. Omit/"" = one flat list. */
  group_by?: SlotListGroupBy;
}

interface SlotListSlot {
  instance_id: number;
  title: string;
  time_range: string;
  status: string;
}

export interface SlotListRow {
  student_id: number;
  name: string;
  school_class: string;
  group_name: string;
  group_id?: number;
  instance_id?: number;
  slot: string;
  room_name?: string;
  pickup_time?: string;
  planned: boolean;
  present: boolean;
  unplanned: boolean;
  excused: boolean;
  status_label: string;
  /** Section heading when grouped; empty for a flat list. */
  group_title?: string;
}

interface SlotListGroupOption {
  id: number;
  name: string;
}

interface SlotListCounters {
  planned: number;
  present: number;
  missing: number;
  excused: number;
  unplanned: number;
}

export interface SlotListResult {
  date: string;
  target: SlotListTarget;
  pickup_cohort?: SlotListPickupCohort;
  list_label: string;
  source: SlotListSource;
  group_by?: SlotListGroupBy;
  provenance: string;
  slots: SlotListSlot[];
  groups: SlotListGroupOption[];
  classes: string[];
  counters: SlotListCounters;
  rows: SlotListRow[];
}

interface SlotListPickupCohortOption {
  cohort: SlotListPickupCohort;
  label: string;
  available: boolean;
  row_count: number;
}

export interface SlotListOptionsResult {
  date: string;
  slots: SlotListSlot[];
  pickup_cohorts: SlotListPickupCohortOption[];
}

export const SLOT_LIST_SOURCE_LABELS: Record<SlotListSource, string> = {
  planned: "Plan",
  actual: "Ist",
  reconciliation: "Abgleich",
};

export const SLOT_LIST_GROUP_BY_LABELS: Record<SlotListGroupBy, string> = {
  "": "Keine",
  slot: "Angebot",
  room: "Raum",
  class: "Klasse",
  pickup_time: "Abholzeit",
};

// Valid grouping options per target — mirrors GroupBy.ValidFor on the backend.
export function groupByOptionsFor(target: SlotListTarget): SlotListGroupBy[] {
  return target === "pickup_cohort"
    ? ["", "class", "pickup_time"]
    : ["", "slot", "room", "class"];
}

export async function fetchSlotListOptions(
  date: string,
): Promise<SlotListOptionsResult> {
  const response = await fetch("/api/timetable/lists/options", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ date }),
  });
  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }
  const payload = (await response.json()) as { data: SlotListOptionsResult };
  return payload.data;
}

export async function fetchSlotListPreview(
  request: SlotListRequest,
): Promise<SlotListResult> {
  const response = await fetch("/api/timetable/lists/preview", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });
  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }
  // route-wrapper envelopes handler results as { data, status }
  const payload = (await response.json()) as { data: SlotListResult };
  return payload.data;
}

export type SlotListExportMode = "download" | "print";

export async function exportSlotList(
  request: SlotListRequest,
  format: SlotListFormat,
  mode: SlotListExportMode,
): Promise<void> {
  const response = await fetch("/api/timetable/lists/export", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ...request, format }),
  });
  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }

  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  if (mode === "print") {
    openForPrint(url);
    return;
  }

  const link = document.createElement("a");
  link.href = url;
  link.download =
    filenameFromDisposition(response) ?? fallbackFilename(request, format);
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function fallbackFilename(
  request: SlotListRequest,
  format: SlotListFormat,
): string {
  const source =
    request.source === "planned"
      ? "plan"
      : request.source === "actual"
        ? "ist"
        : "abgleich";
  const target =
    request.target === "pickup_cohort"
      ? request.pickup_cohort === "long_day"
        ? "ganztag-1600"
        : "ganztag-1430"
      : "angebote";
  return `tagesliste-${source}-${target}-${request.date}.${format}`;
}

function openForPrint(url: string) {
  const target = globalThis.open(url, "_blank");
  if (!target) {
    URL.revokeObjectURL(url);
    throw new Error("Der Druckdialog konnte nicht geöffnet werden.");
  }
  globalThis.setTimeout(() => {
    target.focus();
    target.print();
    URL.revokeObjectURL(url);
  }, 250);
}

function filenameFromDisposition(response: Response): string | null {
  const disposition = response.headers.get("content-disposition");
  const match = /filename="([^"]+)"/.exec(disposition ?? "");
  return match?.[1] ?? null;
}

async function readErrorMessage(response: Response): Promise<string> {
  try {
    const payload = (await response.json()) as { error?: string };
    if (payload.error) return payload.error;
  } catch {
    // fall through to generic message
  }
  return "Die Liste konnte nicht geladen werden.";
}

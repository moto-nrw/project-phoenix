// Client API + types for slot lists (issue #1565): lists from selected
// timetable slots or Ganztag pickup cohorts with three data modes
// (Plan / Ist / Abgleich) and PDF/XLSX export.

export type SlotListTarget = "slots" | "pickup_cohort";

export type SlotListPickupCohort = "short_day" | "long_day";

export type SlotListKind =
  "edge_hours" | "learning_time" | "activity" | "mensa";

export type SlotListSource = "planned" | "actual" | "reconciliation";

export type SlotListGroupBy = "" | "slot" | "room" | "class" | "pickup_time";

export type SlotListFormat = "pdf" | "xlsx";

export interface SlotListRequest {
  date: string; // YYYY-MM-DD
  target: SlotListTarget;
  pickup_cohort?: SlotListPickupCohort;
  list_kind?: SlotListKind;
  source: SlotListSource;
  /** Selected timetable slots. Omit = all selectable slots; [] = none selected. */
  instance_ids?: string[];
  /** Optional education-group filter. Omit/empty = all groups. */
  group_ids?: string[];
  /** Optional school-class filter. Omit/empty = all classes. */
  classes?: string[];
  /** Section the preview/export. Omit/"" = one flat list. */
  group_by?: SlotListGroupBy;
  /**
   * Content hash (SlotListResult.signature) of the preview the user reviewed.
   * Export-only: the backend refuses with 409 when its fresh rebuild no longer
   * matches, so a downloaded file never differs from the approved preview.
   */
  expected_signature?: string;
}

interface SlotListSlot {
  instance_id: string;
  title: string;
  time_range: string;
  status: string;
  /** Timetable list classification; drives which classified list a slot exports to. */
  list_kind?: string;
  /** Planned room name; reshapes a room-grouped export when it changes. */
  room_name?: string;
}

export interface SlotListRow {
  student_id: string;
  name: string;
  school_class: string;
  group_name: string;
  group_id?: string;
  instance_id?: string;
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
  id: string;
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
  list_kind?: SlotListKind;
  list_label: string;
  source: SlotListSource;
  group_by?: SlotListGroupBy;
  provenance: string;
  slots: SlotListSlot[];
  groups: SlotListGroupOption[];
  classes: string[];
  counters: SlotListCounters;
  rows: SlotListRow[];
  /**
   * Backend content hash of this rendered list. Echoed back as an export's
   * expected_signature so the backend can refuse (409) an export whose fresh
   * rebuild no longer matches the reviewed preview (#1565 review pass 2).
   */
  signature: string;
}

interface SlotListPickupCohortOption {
  cohort: SlotListPickupCohort;
  label: string;
  available: boolean;
  row_count: number;
}

interface SlotListKindOption {
  kind: SlotListKind;
  label: string;
  available: boolean;
  slot_count: number;
  row_count: number;
}

export interface SlotListOptionsResult {
  date: string;
  slots: SlotListSlot[];
  pickup_cohorts: SlotListPickupCohortOption[];
  list_kinds: SlotListKindOption[];
}

export const SLOT_LIST_KIND_LABELS: Record<SlotListKind, string> = {
  edge_hours: "Randstunden",
  learning_time: "Lernzeit",
  activity: "AG-Angebote",
  mensa: "Mensa",
};

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

/**
 * Error thrown by exportSlotList for a non-OK export response. Carries the HTTP
 * status so callers can branch on the backend's 409 drift refusal (the fresh
 * build no longer matches the reviewed preview) without string-matching the
 * message (#1565 review pass 2).
 */
export class SlotListExportError extends Error {
  readonly status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "SlotListExportError";
    this.status = status;
  }
}

// Thrown when the caller's commitGuard reports the live selection no longer
// matches the exported request AFTER the file has been built but BEFORE it is
// handed out. Distinct from SlotListExportError so the caller can show a
// "selection changed" hint instead of logging an export failure (#1565 review
// pass 10).
export class SlotListExportSupersededError extends Error {
  constructor() {
    super("slot list export superseded by a newer selection");
    this.name = "SlotListExportSupersededError";
  }
}

export async function exportSlotList(
  request: SlotListRequest,
  format: SlotListFormat,
  mode: SlotListExportMode,
  preOpenedPrintTarget?: Window | null,
  expectedSignature?: string,
  // Evaluated after the export response arrives but before the file is handed
  // out (download/print). Returning false aborts the handoff — the caller's
  // selection moved on during the export round-trip, so the built file is stale.
  commitGuard?: () => boolean,
): Promise<void> {
  // The print tab must open synchronously while the click's transient user
  // activation is still valid: after `await fetch` popup blockers (Safari,
  // strict settings) return null from window.open and Drucken always fails.
  // When the caller runs its own async step before export (e.g. the pre-export
  // slot revalidation), it opens the placeholder during the click and hands it
  // in via preOpenedPrintTarget; we reuse it rather than opening a second tab
  // after the activation has lapsed. When no tab is supplied we open one now.
  const printTarget =
    mode === "print"
      ? preOpenedPrintTarget !== undefined
        ? preOpenedPrintTarget
        : globalThis.open("", "_blank")
      : null;
  if (mode === "print" && !printTarget) {
    throw new Error("Der Druckdialog konnte nicht geöffnet werden.");
  }

  try {
    const response = await fetch("/api/timetable/lists/export", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        ...request,
        format,
        expected_signature: expectedSignature,
      }),
    });
    if (!response.ok) {
      throw new SlotListExportError(
        response.status,
        await readErrorMessage(response),
      );
    }

    const blob = await response.blob();

    // Last gate before the file leaves the building: the backend already
    // verified the signature, but the export round-trip is itself a window in
    // which the user can change the date/filters. If the live selection no
    // longer matches the exported request, abort the handoff rather than
    // download/print a document for a selection the UI has already left. The
    // catch below closes the print tab and rethrows for the caller to surface
    // (#1565 review pass 10).
    if (commitGuard && !commitGuard()) {
      throw new SlotListExportSupersededError();
    }

    const url = URL.createObjectURL(blob);
    if (printTarget) {
      openForPrint(printTarget, url);
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
  } catch (error) {
    printTarget?.close();
    throw error;
  }
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
      : request.list_kind
        ? request.list_kind
        : "angebote";
  return `tagesliste-${source}-${target}-${request.date}.${format}`;
}

function openForPrint(target: Window, url: string) {
  target.location.href = url;
  // The PDF viewer document fires no reliable load event cross-browser, so
  // printing keeps the pragmatic delay the other print exports use.
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
    const payload = (await response.json()) as {
      error?: string;
      message?: string;
    };
    const raw = payload.error ?? payload.message;
    if (raw) return unwrapBackendMessage(raw);
  } catch {
    // fall through to generic message
  }
  return "Die Liste konnte nicht geladen werden.";
}

// The proxy layers forward a backend error in one of a few shapes: the clean
// German sentence, the raw backend JSON body (`{"status":"error","error":"…"}`)
// when a route bypasses the shared JSON wrapper (the export download route), or
// the legacy `API error (400): {…}` envelope. Peel all of them back to the
// backend's human sentence so a refusal (e.g. a past Ganztag date or a future
// reconciliation) never surfaces an internal prefix or serialized JSON to the
// user (#1565 review pass 2). A plain sentence with no embedded object is
// returned untouched.
function unwrapBackendMessage(raw: string): string {
  const trimmed = raw.trim();
  const jsonStart = trimmed.indexOf("{");
  if (jsonStart === -1) return trimmed;
  try {
    const nested = JSON.parse(trimmed.slice(jsonStart)) as {
      error?: string;
      message?: string;
    };
    return nested.error ?? nested.message ?? trimmed;
  } catch {
    return trimmed;
  }
}

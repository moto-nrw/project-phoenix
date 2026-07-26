import { createLogger } from "~/lib/logger";
import { readEnrollmentError } from "~/lib/enrollment-error-messages";
import type { ChildStatus } from "~/lib/enrollment-admin-api";

const logger = createLogger({ component: "EnrollmentReportAPI" });

export type EnrollmentReportStatus = ChildStatus | "all";
export type EnrollmentReportFormat = "pdf" | "docx" | "xlsx";

export interface CareUsageFilters {
  phase_id: string;
  status?: EnrollmentReportStatus;
  care_offering_ids?: string[];
  day_count?: number;
  grade_level?: number;
  weekday?: string;
  pickup_time?: string;
  search?: string;
}

export interface CareUsageReport {
  phase: {
    id: string;
    name: string;
  };
  filters: {
    phase_id: string;
    status: EnrollmentReportStatus;
    care_offering_ids: string[];
    day_count?: number;
    grade_level?: number;
    weekday?: string;
    pickup_time?: string;
    search?: string;
  };
  totals: {
    children: number;
    by_day_count: Record<string, number>;
    by_weekday_pickup_time: Record<string, Record<string, number>>;
  };
  by_offering: CareUsageOfferingStat[];
  filter_options: {
    offerings: CareUsageOfferingOption[];
    grade_levels: number[];
    pickup_times: string[];
  };
  rows: CareUsageRow[];
}

interface CareUsageOfferingOption {
  id: string;
  name: string;
  counts_as_care: boolean;
}

interface CareUsageOfferingStat {
  offering_id: string;
  offering_name: string;
  children: number;
  by_day_count: Record<string, number>;
}

export interface CareUsageRow {
  request_id: string;
  child_id: string;
  child_first_name: string;
  child_last_name: string;
  date_of_birth: string;
  target_grade_level?: number;
  /** Concrete future class (e.g. "2a") when collected (#1833). */
  target_school_class?: string;
  status: ChildStatus;
  offerings: CareUsageRowOffering[];
  effective_days: string[];
  day_count: number;
  pickup_by_day: Record<string, string>;
  guardian_first_name: string;
  guardian_last_name: string;
  guardian_email: string;
  guardian_phone?: string | null;
  submitted_at: string;
}

interface CareUsageRowOffering {
  id: string;
  name: string;
  days: string[];
  days_source: "selected" | "available";
  days_of_week_mode: string;
  manual_selected_days?: string[];
  automatic_selected_days?: string[];
}

interface BackendEnvelope<T> {
  status?: string;
  data?: T;
  error?: string;
  message?: string;
}

async function readJSON<T>(response: Response): Promise<T> {
  const raw = (await response.json()) as BackendEnvelope<T>;
  if (
    raw &&
    typeof raw === "object" &&
    "data" in raw &&
    raw.data !== undefined
  ) {
    return raw.data as T;
  }
  return raw as unknown as T;
}

async function readError(response: Response, fallback: string): Promise<Error> {
  return readEnrollmentError(
    response,
    fallback,
    logger,
    "enrollment_report_request_failed",
  );
}

export async function getCareUsageReport(
  filters: CareUsageFilters,
): Promise<CareUsageReport> {
  const url = new URL(
    "/api/enrollment/admin/reports/care-usage",
    globalThis.window?.location.origin ?? "http://localhost",
  );
  appendCareUsageParams(url, filters);
  const response = await fetch(`${url.pathname}${url.search}`, {
    cache: "no-store",
  });
  if (!response.ok) {
    throw await readError(response, "Auswertung konnte nicht geladen werden");
  }
  return readJSON<CareUsageReport>(response);
}

export async function exportCareUsageReport(
  filters: CareUsageFilters,
  format: EnrollmentReportFormat,
): Promise<void> {
  const response = await fetch(
    "/api/enrollment/admin/reports/care-usage/export",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ format, filters }),
    },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Auswertung konnte nicht exportiert werden",
    );
  }
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download =
    filenameFromDisposition(response) ?? `anmelde-auswertung.${format}`;
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export async function exportPhaseClassRoster(
  phaseId: string,
  schoolClass: string | null,
  format: EnrollmentReportFormat,
): Promise<void> {
  const response = await fetch(
    "/api/enrollment/admin/reports/class-roster/export",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        format,
        filters: {
          phase_id: phaseId,
          ...(schoolClass
            ? { school_class: schoolClass }
            : { all_classes: true }),
        },
      }),
    },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Klassenliste konnte nicht exportiert werden",
    );
  }
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.download =
    filenameFromDisposition(response) ??
    (schoolClass ? `klassenliste.${format}` : `klassenlisten.${format}`);
  link.href = url;
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function appendCareUsageParams(url: URL, filters: CareUsageFilters) {
  url.searchParams.set("phase_id", String(filters.phase_id));
  if (filters.status) url.searchParams.set("status", filters.status);
  if (filters.care_offering_ids !== undefined) {
    if (filters.care_offering_ids.length === 0) {
      url.searchParams.set("care_offering_ids", "");
    } else {
      for (const id of filters.care_offering_ids) {
        url.searchParams.append("care_offering_ids", String(id));
      }
    }
  }
  if (filters.day_count !== undefined) {
    url.searchParams.set("day_count", String(filters.day_count));
  }
  if (filters.grade_level) {
    url.searchParams.set("grade_level", String(filters.grade_level));
  }
  if (filters.weekday) {
    url.searchParams.set("weekday", filters.weekday);
  }
  if (filters.pickup_time) {
    url.searchParams.set("pickup_time", filters.pickup_time);
  }
  if (filters.search?.trim()) {
    url.searchParams.set("search", filters.search.trim());
  }
}

function filenameFromDisposition(response: Response): string | null {
  const disposition = response.headers.get("content-disposition");
  const match = /filename="([^"]+)"/.exec(disposition ?? "");
  return match?.[1] ?? null;
}

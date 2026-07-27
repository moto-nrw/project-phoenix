// DATEV export preflight (#1417 2b): the report the export dialog shows
// before a DATEV download. Same permission as the export itself
// (time_tracking:manage); the file build and this report share one backend
// code path, so they cannot disagree.

import { sessionFetch } from "./session-cache";

export type DatevFormat = "datev_lodas" | "datev_lug";

interface BackendDatevSkippedStaff {
  last_name: string;
  first_name: string;
  reason: string;
}

interface BackendDatevExportReport {
  format: string;
  year: number;
  month: number;
  line_count: number;
  staff_exported: number;
  staff_skipped: BackendDatevSkippedStaff[];
  unconfigured_categories: string[];
  open_month: boolean;
}

interface DatevSkippedStaff {
  lastName: string;
  firstName: string;
  reason: string;
}

export interface DatevExportReport {
  lineCount: number;
  staffExported: number;
  staffSkipped: DatevSkippedStaff[];
  unconfiguredCategories: string[];
  openMonth: boolean;
}

/** Thrown for the 409 "payroll configuration incomplete" preflight answer. */
export class DatevConfigIncompleteError extends Error {
  constructor() {
    super("payroll_config_incomplete");
    this.name = "DatevConfigIncompleteError";
  }
}

function mapDatevExportReport(
  data: BackendDatevExportReport,
): DatevExportReport {
  return {
    lineCount: data.line_count,
    staffExported: data.staff_exported,
    staffSkipped: (data.staff_skipped ?? []).map((entry) => ({
      lastName: entry.last_name,
      firstName: entry.first_name,
      reason: entry.reason,
    })),
    unconfiguredCategories: data.unconfigured_categories ?? [],
    openMonth: data.open_month,
  };
}

export async function fetchDatevExportReport(
  year: number,
  month: number,
  format: DatevFormat,
): Promise<DatevExportReport> {
  const params = new URLSearchParams({
    year: String(year),
    month: String(month),
    format,
  });
  const response = await sessionFetch(
    `/api/staff/time-tracking/export/datev-report?${params.toString()}`,
  );
  if (response.status === 409) {
    throw new DatevConfigIncompleteError();
  }
  if (!response.ok) {
    throw new Error(`Failed to fetch DATEV report: ${response.statusText}`);
  }
  const json = (await response.json()) as { data: BackendDatevExportReport };
  return mapDatevExportReport(json.data);
}

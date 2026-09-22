import { operatorFetch, OperatorApiError } from "./api-helpers";
import { downloadBlob, filenameFromDisposition } from "~/lib/file-download";

/**
 * Operator billing report (#2791): the key day that applies to every school
 * and the monthly key-date counts the backend captures once per month.
 */

interface BackendBillingKeyDay {
  key_day: number;
  next_key_date: string;
  updated_at: string;
}

interface BackendBillingKeyDateCount {
  school_id: number;
  school_name: string;
  organization_name: string;
  period: string;
  key_date: string;
  active_students: number;
  active_terminals: number;
  recorded_at: string;
}

export interface BillingKeyDay {
  keyDay: number;
  /** Next key date as YYYY-MM-DD. */
  nextKeyDate: string;
  updatedAt: string;
}

export interface BillingKeyDateCount {
  schoolId: string;
  schoolName: string;
  organizationName: string;
  /** First day of the month as YYYY-MM-DD. */
  period: string;
  /** Key date as YYYY-MM-DD. */
  keyDate: string;
  activeStudents: number;
  activeTerminals: number;
  /** Instant of the capture (RFC 3339). */
  recordedAt: string;
}

/** The key day may be 1 to 28, so every month has it. */
export const BILLING_KEY_DAY_MIN = 1;
export const BILLING_KEY_DAY_MAX = 28;

function mapBillingKeyDay(data: BackendBillingKeyDay): BillingKeyDay {
  return {
    keyDay: data.key_day,
    nextKeyDate: data.next_key_date,
    updatedAt: data.updated_at,
  };
}

export function mapBillingKeyDateCount(
  data: BackendBillingKeyDateCount,
): BillingKeyDateCount {
  return {
    schoolId: String(data.school_id),
    schoolName: data.school_name,
    organizationName: data.organization_name,
    period: data.period,
    keyDate: data.key_date,
    activeStudents: data.active_students,
    activeTerminals: data.active_terminals,
    recordedAt: data.recorded_at,
  };
}

/** YYYY-MM of a period (YYYY-MM-01). */
export function billingMonth(period: string): string {
  return period.slice(0, 7);
}

export const operatorBillingService = {
  async getKeyDay(): Promise<BillingKeyDay> {
    const data = await operatorFetch<BackendBillingKeyDay>(
      "/api/operator/billing/key-day",
    );
    return mapBillingKeyDay(data);
  },

  async updateKeyDay(keyDay: number): Promise<BillingKeyDay> {
    const data = await operatorFetch<BackendBillingKeyDay>(
      "/api/operator/billing/key-day",
      { method: "PUT", body: { key_day: keyDay } },
    );
    return mapBillingKeyDay(data);
  },

  async listKeyDateCounts(): Promise<BillingKeyDateCount[]> {
    const data = await operatorFetch<BackendBillingKeyDateCount[] | null>(
      "/api/operator/billing/key-date-counts",
    );
    return (data ?? []).map(mapBillingKeyDateCount);
  },

  /** Downloads the CSV of one month (YYYY-MM), or of every month. */
  async downloadKeyDateCounts(month?: string): Promise<void> {
    const query = month ? `?month=${encodeURIComponent(month)}` : "";
    const response = await fetch(
      `/api/operator/billing/key-date-counts/export${query}`,
      { credentials: "include" },
    );
    if (!response.ok) {
      throw new OperatorApiError(
        "Die Datei konnte nicht erstellt werden.",
        response.status,
      );
    }
    const blob = await response.blob();
    const fallback = month
      ? `stichtagszahlen-${month}.csv`
      : "stichtagszahlen.csv";
    downloadBlob(blob, filenameFromDisposition(response) ?? fallback);
  },
};

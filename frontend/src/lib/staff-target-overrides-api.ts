import { sessionFetch } from "./session-cache";

// Sonderarbeitszeit (#3259): for every Monday to Friday in [startDate,
// endDate] the staff member's daily Soll is dailyMinutes, or with
// weekdayMinutes (#3745, Monday to Friday) the Soll of that weekday. Exactly
// one of the two is set. It wins over closing days and the
// Arbeitszeitmodell; statutory holidays stay at zero.
export interface StaffTargetOverride {
  id: string;
  staffId: string;
  startDate: string;
  endDate: string;
  dailyMinutes: number | null;
  weekdayMinutes: readonly number[] | null;
}

type StaffTargetOverrideInput = {
  startDate: string;
  endDate: string;
} & (
  | { dailyMinutes: number; weekdayMinutes?: never }
  | { weekdayMinutes: readonly number[]; dailyMinutes?: never }
);

interface BackendStaffTargetOverride {
  id: number | string;
  staff_id: number | string;
  start_date: string;
  end_date: string;
  daily_minutes: number | null;
  weekday_minutes?: number[] | null;
}

function mapStaffTargetOverride(
  row: BackendStaffTargetOverride,
): StaffTargetOverride {
  return {
    id: String(row.id),
    staffId: String(row.staff_id),
    startDate: row.start_date.slice(0, 10),
    endDate: row.end_date.slice(0, 10),
    dailyMinutes: row.daily_minutes,
    weekdayMinutes: row.weekday_minutes ?? null,
  };
}

// The backend answers an overlap, a closed month (409) and an invalid range
// (400) with a complete German sentence the user can act on. Technical
// messages (no closing period) fall back to the generic text.
async function errorMessage(
  response: Response,
  fallback: string,
): Promise<string> {
  if (response.status !== 409 && response.status !== 400) return fallback;
  const text = await response.text().catch(() => "");
  try {
    const payload = JSON.parse(text) as { error?: unknown };
    if (typeof payload.error === "string" && payload.error.endsWith(".")) {
      return payload.error;
    }
  } catch {
    // Not JSON: keep the fallback.
  }
  return fallback;
}

const FAILED =
  "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";

function toBody(input: StaffTargetOverrideInput): string {
  return JSON.stringify({
    start_date: input.startDate,
    end_date: input.endDate,
    ...(input.weekdayMinutes
      ? { weekday_minutes: input.weekdayMinutes }
      : { daily_minutes: input.dailyMinutes }),
  });
}

class StaffTargetOverrideService {
  async list(staffId: string): Promise<StaffTargetOverride[]> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/target-overrides`,
    );
    if (!response.ok) {
      throw new Error(
        `Failed to fetch target overrides: ${response.statusText}`,
      );
    }
    const json = (await response.json()) as {
      data: BackendStaffTargetOverride[] | null;
    };
    return (json.data ?? []).map(mapStaffTargetOverride);
  }

  async create(
    staffId: string,
    input: StaffTargetOverrideInput,
  ): Promise<StaffTargetOverride> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/target-overrides`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: toBody(input),
      },
    );
    if (!response.ok) throw new Error(await errorMessage(response, FAILED));
    const json = (await response.json()) as {
      data: BackendStaffTargetOverride;
    };
    return mapStaffTargetOverride(json.data);
  }

  async delete(staffId: string, overrideId: string): Promise<void> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/target-overrides/${overrideId}`,
      { method: "DELETE" },
    );
    if (!response.ok) throw new Error(await errorMessage(response, FAILED));
  }
}

export const staffTargetOverrideService = new StaffTargetOverrideService();

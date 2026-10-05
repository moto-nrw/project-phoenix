import { apiErrorFromResponse } from "./api-error";
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

const FAILED = "Failed to change target override";

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
      throw await apiErrorFromResponse(
        response,
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
    if (!response.ok) throw await apiErrorFromResponse(response, FAILED);
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
    if (!response.ok) throw await apiErrorFromResponse(response, FAILED);
  }
}

export const staffTargetOverrideService = new StaffTargetOverrideService();

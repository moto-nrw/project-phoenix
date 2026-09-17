// API client for the school's own Abwesenheitsarten (#2403).
//
// These sit next to the five standard types (Urlaub, Krank, Fortbildung,
// Sonstige, Freizeitausgleich), which are code constants on both sides and
// never appear in this list. A school-defined art is a named subtype of
// "Sonstige" and inherits its calculation — the name changes what people read,
// never what the Stundenkonto or das Urlaubskontingent do.
//
// Reads need time_tracking:manage or vacation:approve; writes need
// time_tracking:manage. There is no delete: a used art is deactivated, so it
// stays readable on the absences already filed under it.

import { sessionFetch } from "./session-cache";

interface BackendAbsenceType {
  id: string;
  name: string;
  base_type: string;
  is_active: boolean;
  allowance_enabled: boolean;
  carryover_until?: string | null;
}

export interface AbsenceType {
  /** Backend int64 as a string, matching the repo-wide ID convention. */
  readonly id: string;
  readonly name: string;
  /** The standard type whose calculation this art inherits (today: "other"). */
  readonly baseType: string;
  readonly isActive: boolean;
  /**
   * The art keeps an own yearly account per person. A booking above it is
   * always refused (#3256).
   */
  readonly allowanceEnabled: boolean;
  /**
   * "MM-DD" in the following year until which a rest stays usable, or null
   * when the rest expires on 31.12. (#3257).
   */
  readonly carryoverUntil: string | null;
}

function mapAbsenceType(data: BackendAbsenceType): AbsenceType {
  return {
    id: data.id,
    name: data.name,
    baseType: data.base_type,
    isActive: data.is_active,
    allowanceEnabled: data.allowance_enabled,
    carryoverUntil: data.carryover_until ?? null,
  };
}

class AbsenceTypeApiError extends Error {
  readonly status: number;
  readonly detail: string;

  constructor(status: number, detail: string) {
    // The detail is the server's German reason; it is shown as-is.
    super(detail);
    this.name = "AbsenceTypeApiError";
    this.status = status;
    this.detail = detail;
  }
}

async function readError(
  response: Response,
  fallback: string,
): Promise<AbsenceTypeApiError> {
  let detail = "";
  const contentType = response.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) {
    try {
      const body = (await response.json()) as { error?: string };
      detail = body.error ?? "";
    } catch {
      detail = "";
    }
  } else {
    detail = await response.text();
  }
  return new AbsenceTypeApiError(response.status, detail || fallback);
}

async function readList(response: Response): Promise<AbsenceType[]> {
  if (!response.ok) {
    throw await readError(
      response,
      "Abwesenheitsarten konnten nicht geladen werden",
    );
  }
  const json = (await response.json()) as {
    data: BackendAbsenceType[] | null;
  };
  return (json.data ?? []).map(mapAbsenceType);
}

async function readOne(response: Response): Promise<AbsenceType> {
  if (!response.ok) {
    throw await readError(
      response,
      "Abwesenheitsart konnte nicht gespeichert werden",
    );
  }
  const json = (await response.json()) as { data: BackendAbsenceType };
  return mapAbsenceType(json.data);
}

class AbsenceTypeService {
  async getAbsenceTypes(): Promise<AbsenceType[]> {
    const response = await sessionFetch("/api/staff/absence-types");
    return readList(response);
  }

  async createAbsenceType(
    name: string,
    config: {
      allowanceEnabled: boolean;
      carryoverUntil?: string | null;
    },
  ): Promise<AbsenceType> {
    const response = await sessionFetch("/api/staff/absence-types", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name,
        allowance_enabled: config.allowanceEnabled,
        ...(config.carryoverUntil !== undefined
          ? { carryover_until: config.carryoverUntil ?? "" }
          : {}),
      }),
    });
    return readOne(response);
  }

  /** Renames and/or (de)activates. Omitted fields stay as they are. */
  async updateAbsenceType(
    id: string,
    changes: {
      name?: string;
      isActive?: boolean;
      allowanceEnabled?: boolean;
      carryoverUntil?: string | null;
    },
  ): Promise<AbsenceType> {
    const response = await sessionFetch(`/api/staff/absence-types/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        ...(changes.name !== undefined ? { name: changes.name } : {}),
        ...(changes.isActive !== undefined
          ? { is_active: changes.isActive }
          : {}),
        ...(changes.allowanceEnabled !== undefined
          ? { allowance_enabled: changes.allowanceEnabled }
          : {}),
        ...(changes.carryoverUntil !== undefined
          ? { carryover_until: changes.carryoverUntil ?? "" }
          : {}),
      }),
    });
    return readOne(response);
  }

  async getAllowance(
    absenceTypeId: string,
    staffId: string,
    year: number,
  ): Promise<AbsenceTypeAllowanceSummary> {
    const response = await sessionFetch(
      `/api/staff/absence-types/${absenceTypeId}/allowances/${staffId}?year=${year}`,
    );
    return readAllowance(response);
  }

  /**
   * Which yearly Kontingent a planned booking uses. `blocked` is true when
   * one of them would drop below zero; the booking is then refused.
   */
  async previewAllowance(
    absenceTypeId: string,
    staffId: string,
    booking: { dateStart: string; dateEnd: string; halfDay: boolean },
  ): Promise<AbsenceTypeAllowancePreview> {
    const query = new URLSearchParams({
      date_start: booking.dateStart,
      date_end: booking.dateEnd,
      half_day: String(booking.halfDay),
    });
    const response = await sessionFetch(
      `/api/staff/absence-types/${absenceTypeId}/allowances/${staffId}/preview?${query.toString()}`,
    );
    if (!response.ok) {
      throw await readError(
        response,
        "Die Vorschau konnte nicht berechnet werden",
      );
    }
    const json = (await response.json()) as {
      data: {
        years: BackendAbsenceTypeAllowanceSummary[] | null;
        blocked: boolean;
      };
    };
    return {
      years: (json.data.years ?? []).map(mapAllowance),
      blocked: json.data.blocked,
    };
  }

  async setAllowance(
    absenceTypeId: string,
    staffId: string,
    payload: { year: number; entitledDays: number; reason: string },
  ): Promise<AbsenceTypeAllowanceSummary> {
    const response = await sessionFetch(
      `/api/staff/absence-types/${absenceTypeId}/allowances/${staffId}`,
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          year: payload.year,
          entitled_days: payload.entitledDays,
          reason: payload.reason,
        }),
      },
    );
    return readAllowance(response);
  }
}

interface BackendAbsenceTypeAllowanceSummary {
  staff_id: string;
  absence_type_id: string;
  year: number;
  entitled_days: number;
  taken_days: number;
  reserved_days: number;
  remaining_days: number;
  expires_on?: string;
  expired_days?: number;
  booking_days?: number;
  carried_in?: {
    year: number;
    remaining_days: number;
    expired_days: number;
    expires_on: string;
  } | null;
}

/** The previous year's rest as seen from the year it is carried into. */
interface AbsenceTypeAllowanceCarry {
  readonly year: number;
  readonly remainingDays: number;
  readonly expiredDays: number;
  readonly expiresOn: string;
}

export interface AbsenceTypeAllowancePreview {
  readonly years: readonly AbsenceTypeAllowanceSummary[];
  readonly blocked: boolean;
}

export interface AbsenceTypeAllowanceSummary {
  readonly staffId: string;
  readonly absenceTypeId: string;
  readonly year: number;
  readonly entitledDays: number;
  readonly takenDays: number;
  readonly reservedDays: number;
  readonly remainingDays: number;
  /** Last day (YYYY-MM-DD) the rest can be booked. */
  readonly expiresOn: string;
  /** Rest left when expiresOn passed; shown, never dropped. */
  readonly expiredDays: number;
  /** On previews: what the booking takes from this year. */
  readonly bookingDays: number;
  readonly carriedIn: AbsenceTypeAllowanceCarry | null;
}

function mapAllowance(
  data: BackendAbsenceTypeAllowanceSummary,
): AbsenceTypeAllowanceSummary {
  return {
    staffId: data.staff_id,
    absenceTypeId: data.absence_type_id,
    year: data.year,
    entitledDays: data.entitled_days,
    takenDays: data.taken_days,
    reservedDays: data.reserved_days,
    remainingDays: data.remaining_days,
    expiresOn: data.expires_on ?? `${data.year}-12-31`,
    expiredDays: data.expired_days ?? 0,
    bookingDays: data.booking_days ?? 0,
    carriedIn: data.carried_in
      ? {
          year: data.carried_in.year,
          remainingDays: data.carried_in.remaining_days,
          expiredDays: data.carried_in.expired_days,
          expiresOn: data.carried_in.expires_on,
        }
      : null,
  };
}

async function readAllowance(
  response: Response,
): Promise<AbsenceTypeAllowanceSummary> {
  if (!response.ok) {
    throw await readError(
      response,
      "Kontingent konnte nicht gespeichert werden",
    );
  }
  const json = (await response.json()) as {
    data: BackendAbsenceTypeAllowanceSummary;
  };
  return mapAllowance(json.data);
}

export const absenceTypeService = new AbsenceTypeService();

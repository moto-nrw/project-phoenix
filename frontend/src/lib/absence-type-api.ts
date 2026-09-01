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
  overrun_policy: "warn" | "block";
}

export interface AbsenceType {
  /** Backend int64 as a string, matching the repo-wide ID convention. */
  readonly id: string;
  readonly name: string;
  /** The standard type whose calculation this art inherits (today: "other"). */
  readonly baseType: string;
  readonly isActive: boolean;
  readonly allowanceEnabled: boolean;
  readonly overrunPolicy: "warn" | "block";
}

function mapAbsenceType(data: BackendAbsenceType): AbsenceType {
  return {
    id: data.id,
    name: data.name,
    baseType: data.base_type,
    isActive: data.is_active,
    allowanceEnabled: data.allowance_enabled,
    overrunPolicy: data.overrun_policy,
  };
}

class AbsenceTypeApiError extends Error {
  readonly status: number;
  readonly detail: string;

  constructor(status: number, detail: string) {
    super(`HTTP ${status}: ${detail}`);
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

  async createAbsenceType(name: string): Promise<AbsenceType> {
    const response = await sessionFetch("/api/staff/absence-types", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
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
      overrunPolicy?: "warn" | "block";
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
        ...(changes.overrunPolicy !== undefined
          ? { overrun_policy: changes.overrunPolicy }
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
}

export interface AbsenceTypeAllowanceSummary {
  readonly staffId: string;
  readonly absenceTypeId: string;
  readonly year: number;
  readonly entitledDays: number;
  readonly takenDays: number;
  readonly reservedDays: number;
  readonly remainingDays: number;
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
  return {
    staffId: json.data.staff_id,
    absenceTypeId: json.data.absence_type_id,
    year: json.data.year,
    entitledDays: json.data.entitled_days,
    takenDays: json.data.taken_days,
    reservedDays: json.data.reserved_days,
    remainingDays: json.data.remaining_days,
  };
}

export const absenceTypeService = new AbsenceTypeService();

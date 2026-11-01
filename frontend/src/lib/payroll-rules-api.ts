// Eigene Lohnarten und Zuschläge (#4807): the school's own wage types,
// surcharge rules for night, Sunday and holiday work and the wage type of its
// own absence types. Maintained on /payroll; requires config:manage. The
// backend writes every change into the settings audit.

import { apiErrorFromResponse } from "./api-error";
import { sessionFetch } from "./session-cache";

export type WageTypeUnit = "stunden" | "tage" | "betrag";
export type SurchargeKind = "nacht" | "sonntag" | "feiertag";
export type SurchargeRateType = "prozent" | "betrag";

export interface PayrollWageType {
  id: string;
  name: string;
  number: string;
  unit: WageTypeUnit;
}

export interface PayrollSurchargeRule {
  id: string;
  kind: SurchargeKind;
  /** Minutes after midnight; set for night rules only. */
  windowStartMinute: number | null;
  windowEndMinute: number | null;
  rateType: SurchargeRateType;
  /** Percent × 100 (25 % = 2500) or cent per hour. */
  rateHundredths: number;
  wageTypeId: string;
}

export interface PayrollAbsenceTypeMapping {
  id: string;
  name: string;
  isActive: boolean;
  /** `null`: the absence type keeps the category it had. */
  wageTypeId: string | null;
}

export interface PayrollRules {
  wageTypes: PayrollWageType[];
  surchargeRules: PayrollSurchargeRule[];
  absenceTypes: PayrollAbsenceTypeMapping[];
}

interface BackendPayrollRules {
  wage_types: { id: string; name: string; number: string; unit: string }[];
  surcharge_rules: {
    id: string;
    kind: string;
    window_start_minute: number | null;
    window_end_minute: number | null;
    rate_type: string;
    rate_hundredths: number;
    wage_type_id: string;
  }[];
  absence_types: {
    id: string;
    name: string;
    is_active: boolean;
    wage_type_id: string | null;
  }[];
}

const UNITS: readonly WageTypeUnit[] = ["stunden", "tage", "betrag"];
const KINDS: readonly SurchargeKind[] = ["nacht", "sonntag", "feiertag"];

function unitOf(value: string): WageTypeUnit {
  return UNITS.find((unit) => unit === value) ?? "stunden";
}

function kindOf(value: string): SurchargeKind {
  return KINDS.find((kind) => kind === value) ?? "nacht";
}

export function mapPayrollRules(data: BackendPayrollRules): PayrollRules {
  return {
    wageTypes: data.wage_types.map((wageType) => ({
      id: wageType.id,
      name: wageType.name,
      number: wageType.number,
      unit: unitOf(wageType.unit),
    })),
    surchargeRules: data.surcharge_rules.map((rule) => ({
      id: rule.id,
      kind: kindOf(rule.kind),
      windowStartMinute: rule.window_start_minute,
      windowEndMinute: rule.window_end_minute,
      rateType: rule.rate_type === "betrag" ? "betrag" : "prozent",
      rateHundredths: rule.rate_hundredths,
      wageTypeId: rule.wage_type_id,
    })),
    absenceTypes: data.absence_types.map((absenceType) => ({
      id: absenceType.id,
      name: absenceType.name,
      isActive: absenceType.is_active,
      wageTypeId: absenceType.wage_type_id,
    })),
  };
}

const BASE = "/api/staff/time-tracking/payroll-rules";

async function send(
  path: string,
  method: "POST" | "PUT" | "DELETE",
  body: unknown,
  failure: string,
): Promise<void> {
  const response = await sessionFetch(`${BASE}${path}`, {
    method,
    headers:
      body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!response.ok) {
    throw await apiErrorFromResponse(response, failure);
  }
}

export async function fetchPayrollRules(): Promise<PayrollRules> {
  const response = await sessionFetch(BASE);
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `Failed to fetch payroll rules: ${response.statusText}`,
    );
  }
  const json = (await response.json()) as { data: BackendPayrollRules };
  return mapPayrollRules(json.data);
}

export interface WageTypeInput {
  name: string;
  number: string;
  unit: WageTypeUnit;
}

export function createWageType(input: WageTypeInput): Promise<void> {
  return send("/wage-types", "POST", input, "Creating the wage type failed");
}

export function updateWageType(
  id: string,
  input: WageTypeInput,
): Promise<void> {
  return send(
    `/wage-types/${encodeURIComponent(id)}`,
    "PUT",
    input,
    "Updating the wage type failed",
  );
}

export function archiveWageType(id: string): Promise<void> {
  return send(
    `/wage-types/${encodeURIComponent(id)}`,
    "DELETE",
    undefined,
    "Archiving the wage type failed",
  );
}

export interface SurchargeRuleInput {
  kind: SurchargeKind;
  windowStartMinute: number | null;
  windowEndMinute: number | null;
  rateType: SurchargeRateType;
  rateHundredths: number;
  wageTypeId: string;
}

function ruleBody(input: SurchargeRuleInput) {
  const night = input.kind === "nacht";
  return {
    kind: input.kind,
    window_start_minute: night ? input.windowStartMinute : null,
    window_end_minute: night ? input.windowEndMinute : null,
    rate_type: input.rateType,
    rate_hundredths: input.rateHundredths,
    wage_type_id: input.wageTypeId,
  };
}

export function createSurchargeRule(input: SurchargeRuleInput): Promise<void> {
  return send(
    "/surcharge-rules",
    "POST",
    ruleBody(input),
    "Creating the surcharge rule failed",
  );
}

export function updateSurchargeRule(
  id: string,
  input: SurchargeRuleInput,
): Promise<void> {
  return send(
    `/surcharge-rules/${encodeURIComponent(id)}`,
    "PUT",
    ruleBody(input),
    "Updating the surcharge rule failed",
  );
}

export function archiveSurchargeRule(id: string): Promise<void> {
  return send(
    `/surcharge-rules/${encodeURIComponent(id)}`,
    "DELETE",
    undefined,
    "Archiving the surcharge rule failed",
  );
}

export function setAbsenceTypeWageType(
  absenceTypeId: string,
  wageTypeId: string | null,
): Promise<void> {
  return send(
    `/absence-types/${encodeURIComponent(absenceTypeId)}`,
    "PUT",
    { wage_type_id: wageTypeId },
    "Setting the wage type of the absence type failed",
  );
}

// ─── Wording and number formats ─────────────────────────────────────────────

export const WAGE_TYPE_UNIT_LABELS: Record<WageTypeUnit, string> = {
  stunden: "Stunden",
  tage: "Tage",
  betrag: "Betrag in Euro",
};

export const SURCHARGE_KIND_LABELS: Record<SurchargeKind, string> = {
  nacht: "Nachtarbeit",
  sonntag: "Sonntagsarbeit",
  feiertag: "Feiertagsarbeit",
};

export const SURCHARGE_RATE_TYPE_LABELS: Record<SurchargeRateType, string> = {
  prozent: "Prozent vom Stundenlohn",
  betrag: "Euro je Stunde",
};

/** A percent rule books hours, an amount rule an amount (backend rule). */
export function wageTypeUnitForRate(rateType: SurchargeRateType): WageTypeUnit {
  return rateType === "betrag" ? "betrag" : "stunden";
}

/** "2500" hundredths → "25", "250" → "2,50"; percent drops zero cents. */
export function formatRate(
  rateType: SurchargeRateType,
  hundredths: number,
): string {
  const whole = Math.trunc(hundredths / 100);
  const cents = Math.abs(hundredths % 100);
  if (rateType === "prozent" && cents === 0) return String(whole);
  return `${whole},${String(cents).padStart(2, "0")}`;
}

/** "25", "25,5" or "2.50" → hundredths; `null` for anything else. */
export function parseRate(text: string): number | null {
  const match = /^(\d{1,4})(?:[,.](\d{1,2}))?$/.exec(text.trim());
  if (!match) return null;
  const whole = Number(match[1]);
  const cents = Number((match[2] ?? "0").padEnd(2, "0"));
  const value = whole * 100 + cents;
  return value >= 1 && value <= 100000 ? value : null;
}

/** 1200 → "20:00". */
export function minutesToClock(minutes: number): string {
  const hours = Math.floor(minutes / 60);
  return `${String(hours).padStart(2, "0")}:${String(minutes % 60).padStart(2, "0")}`;
}

/** "20:00" → 1200; `null` for anything that is no wall-clock time. */
export function clockToMinutes(text: string): number | null {
  const match = /^([01]?\d|2[0-3]):([0-5]\d)$/.exec(text.trim());
  if (!match) return null;
  return Number(match[1]) * 60 + Number(match[2]);
}

/** The wage types a rule of the rate type may book onto. */
export function wageTypesForRate(
  wageTypes: readonly PayrollWageType[],
  rateType: SurchargeRateType,
): PayrollWageType[] {
  const unit = wageTypeUnitForRate(rateType);
  return wageTypes.filter((wageType) => wageType.unit === unit);
}

/** Absences book hours or days, never an amount. */
export function wageTypesForAbsences(
  wageTypes: readonly PayrollWageType[],
): PayrollWageType[] {
  return wageTypes.filter((wageType) => wageType.unit !== "betrag");
}

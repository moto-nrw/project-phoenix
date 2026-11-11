/**
 * Anmeldephasen und Betreuungsangebote des Trägers (#4704, ADR 0195).
 *
 * Das Träger-Büro pflegt eine Trägerphase mit ihren Trägerangeboten einmal.
 * Eine Vorschau zeigt je Schule, was das Übertragen ändert; erst „Freigeben
 * und übertragen“ legt die Kopie in jeder Schule an oder gleicht sie ab.
 * Ein Feld, das die Schule geändert hat, behält ihren Wert.
 */

import type {
  CareOfferingSelectionMode,
  PhaseAudience,
  PhaseCareOverflowMode,
  PhaseKind,
} from "./enrollment-phase-api";
import type { CareSelectionRule, DaysOfWeekMode } from "./care-offering-api";
import type { CareWeekday } from "./traeger-care-billing-api";
import {
  type BackendTerms,
  type ContractTerms,
  termsFromBackend,
  termsToBackend,
} from "./traeger-care-contracts-api";
import { traegerRequest } from "./traeger-api";

/** Das letzte Ergebnis des Übertragens an eine Schule. */
export type TraegerCatalogOutcome =
  "applied" | "rejected" | "stale" | "withdrawn";

/** Was das Übertragen an einer Schule tut. */
export type TraegerCatalogAction = "create" | "update" | "withdraw";

/**
 * Ein Feld in der Vorschau: neu, ändert sich, bleibt wie die Schule es
 * geändert hat, oder bleibt gleich.
 */
type TraegerFieldState = "new" | "changes" | "keeps" | "unchanged";

export interface TraegerPhaseFields {
  readonly name: string;
  readonly kind: PhaseKind;
  /** YYYY-MM-DD */
  readonly serviceStartDate: string;
  /** YYYY-MM-DD */
  readonly serviceEndDate: string;
  /** RFC 3339 instant or null. */
  readonly enrollmentOpenAt: string | null;
  readonly enrollmentCloseAt: string | null;
  readonly audience: PhaseAudience;
  readonly careOverflowMode: PhaseCareOverflowMode;
  readonly careOfferingSelectionMode: CareOfferingSelectionMode;
}

export interface TraegerPhaseInput extends TraegerPhaseFields {
  /** Alle Schulen des Trägers, auch später hinzukommende. */
  readonly allSchools: boolean;
  /** Die gewählten Schulen; leer, wenn `allSchools` gilt. */
  readonly schoolIds: readonly string[];
}

export interface TraegerCatalogSchool {
  readonly schoolId: string;
  /** Gehört zur Auswahl der Phase. */
  readonly selected: boolean;
  readonly appliedVersion: number | null;
  readonly outcome: TraegerCatalogOutcome | null;
  readonly errorCode: string | null;
  readonly recordedAt: string | null;
}

export interface TraegerOfferingFields {
  readonly name: string;
  readonly description: string | null;
  readonly daysOfWeekMode: DaysOfWeekMode;
  readonly availableDays: readonly CareWeekday[];
  readonly includesHolidayCare: boolean;
  readonly includesLunch: boolean;
  readonly capacity: number | null;
  /** Preis im Anmeldeformular, nur zur Anzeige. */
  readonly priceCents: number | null;
  readonly isRequired: boolean;
  readonly countsAsCare: boolean;
  readonly selectionGroup: string;
  readonly selectionRule: CareSelectionRule;
  readonly sortOrder: number;
  /** Gehzeit je Wochentag, "HH:MM". */
  readonly pickupTimes: Readonly<Partial<Record<CareWeekday, string>>>;
}

/** Ein Preis: je Wochentag oder je Zahl der Tage in der Woche. */
export interface TraegerCarePrice {
  readonly weekday: CareWeekday | null;
  readonly daysPerWeek: number | null;
  readonly monthlyCents: number;
  /** Endpreis für jedes weitere Kind; null = derselbe Preis. */
  readonly siblingCents: number | null;
  readonly mealExtra: boolean;
}

export interface TraegerOfferingDraft {
  readonly fields: TraegerOfferingFields;
  readonly prices: readonly TraegerCarePrice[];
  readonly contractTemplate: ContractTerms | null;
}

interface TraegerOfferingSchool {
  readonly schoolId: string;
  readonly appliedVersion: number | null;
  readonly outcome: TraegerCatalogOutcome | null;
  readonly errorCode: string | null;
}

export interface TraegerOffering {
  readonly id: string;
  readonly traegerPhaseId: string;
  readonly draft: TraegerOfferingDraft;
  readonly releasedVersion: number | null;
  readonly released: TraegerOfferingDraft | null;
  readonly withdrawnAt: string | null;
  readonly schools: readonly TraegerOfferingSchool[];
}

export interface TraegerPhase {
  readonly id: string;
  readonly draft: TraegerPhaseFields;
  readonly allSchools: boolean;
  readonly releasedVersion: number | null;
  readonly released: TraegerPhaseFields | null;
  readonly withdrawnAt: string | null;
  readonly schools: readonly TraegerCatalogSchool[];
  readonly updatedAt: string;
}

export interface TraegerPhaseDetail extends TraegerPhase {
  readonly offerings: readonly TraegerOffering[];
}

export interface TraegerFieldChange {
  /** Spalte der Kopie, etwa "name" oder "is_active". */
  readonly field: string;
  readonly state: TraegerFieldState;
  readonly traeger: unknown;
  readonly school: unknown;
}

export interface TraegerOfferingPlan {
  readonly traegerOfferingId: string;
  readonly name: string;
  readonly action: TraegerCatalogAction;
  readonly fields: readonly TraegerFieldChange[];
  /** Ab diesem Tag (Monatsbeginn) gelten neue Preise; null = keine neuen. */
  readonly pricesFrom: string | null;
  /** Das Übertragen schreibt die Vertragsvorlage der Schule. */
  readonly contractTemplate: boolean;
}

export interface TraegerSchoolPlan {
  readonly schoolId: string;
  readonly action: TraegerCatalogAction;
  readonly outcome: "ready" | "rejected";
  readonly errorCode: string | null;
  readonly phase: readonly TraegerFieldChange[];
  readonly offerings: readonly TraegerOfferingPlan[];
}

export interface TraegerCatalogPreview {
  readonly traegerPhaseId: string;
  readonly version: number;
  readonly fingerprint: string;
  readonly schools: readonly TraegerSchoolPlan[];
}

interface TraegerRolloutSchool {
  readonly schoolId: string;
  readonly outcome: TraegerCatalogOutcome;
  readonly errorCode: string | null;
}

export interface TraegerRollout {
  readonly traegerPhaseId: string;
  readonly version: number;
  readonly schools: readonly TraegerRolloutSchool[];
}

// --- Backend shapes ----------------------------------------------------------

interface BackendPhaseFields {
  name: string;
  kind: PhaseKind;
  service_start_date: string;
  service_end_date: string;
  enrollment_open_at?: string | null;
  enrollment_close_at?: string | null;
  audience?: PhaseAudience | "";
  care_overflow_mode?: PhaseCareOverflowMode | "";
  care_offering_selection_mode?: CareOfferingSelectionMode | "";
}

interface BackendCatalogSchool {
  school_id: number;
  selected: boolean;
  applied_version?: number | null;
  outcome?: TraegerCatalogOutcome | "";
  error_code?: string;
  recorded_at?: string | null;
}

interface BackendOfferingFields {
  name: string;
  description?: string | null;
  days_of_week_mode: DaysOfWeekMode;
  available_days?: CareWeekday[] | null;
  includes_holiday_care: boolean;
  includes_lunch: boolean;
  capacity?: number | null;
  price_cents?: number | null;
  is_required: boolean;
  counts_as_care: boolean;
  selection_group?: string;
  selection_rule?: CareSelectionRule | "";
  sort_order?: number;
  pickup_times?: Partial<Record<CareWeekday, string>> | null;
}

interface BackendPrice {
  weekday?: CareWeekday | "";
  days_per_week?: number | null;
  monthly_cents: number;
  sibling_cents?: number | null;
  meal_extra: boolean;
}

type BackendContractTemplate = Omit<BackendTerms, "term_months"> & {
  term_months?: number | null;
};

interface BackendOfferingDraft {
  fields: BackendOfferingFields;
  prices?: BackendPrice[] | null;
  contract_template?: BackendContractTemplate | null;
}

interface BackendOffering {
  id: number;
  traeger_phase_id: number;
  draft: BackendOfferingDraft;
  released_version?: number | null;
  released?: BackendOfferingDraft | null;
  withdrawn_at?: string | null;
  schools?:
    | {
        school_id: number;
        applied_version?: number | null;
        outcome?: TraegerCatalogOutcome | "";
        error_code?: string;
      }[]
    | null;
}

interface BackendPhase {
  id: number;
  draft: BackendPhaseFields;
  all_schools: boolean;
  released_version?: number | null;
  released?: BackendPhaseFields | null;
  withdrawn_at?: string | null;
  schools?: BackendCatalogSchool[] | null;
  updated_at: string;
  offerings?: BackendOffering[] | null;
}

interface BackendFieldChange {
  field: string;
  state: TraegerFieldState;
  traeger?: unknown;
  school?: unknown;
}

interface BackendPreview {
  traeger_phase_id: number;
  version: number;
  fingerprint: string;
  schools?:
    | {
        school_id: number;
        action: TraegerCatalogAction;
        outcome: "ready" | "rejected";
        error_code?: string;
        phase?: BackendFieldChange[] | null;
        offerings?:
          | {
              traeger_offering_id: number;
              name: string;
              action: TraegerCatalogAction;
              fields?: BackendFieldChange[] | null;
              prices_from?: string | null;
              contract_template: boolean;
            }[]
          | null;
      }[]
    | null;
}

interface BackendRollout {
  traeger_phase_id: number;
  version: number;
  schools?:
    | {
        school_id: number;
        outcome: TraegerCatalogOutcome;
        error_code?: string;
      }[]
    | null;
}

// --- Mapping -----------------------------------------------------------------

function outcomeOf(
  value: TraegerCatalogOutcome | "" | undefined,
): TraegerCatalogOutcome | null {
  return value ? value : null;
}

function phaseFieldsFromBackend(
  fields: BackendPhaseFields,
): TraegerPhaseFields {
  return {
    name: fields.name,
    kind: fields.kind,
    serviceStartDate: fields.service_start_date,
    serviceEndDate: fields.service_end_date,
    enrollmentOpenAt: fields.enrollment_open_at ?? null,
    enrollmentCloseAt: fields.enrollment_close_at ?? null,
    audience: fields.audience || "open",
    careOverflowMode: fields.care_overflow_mode || "waitlist",
    careOfferingSelectionMode:
      fields.care_offering_selection_mode || "optional",
  };
}

function offeringFieldsFromBackend(
  fields: BackendOfferingFields,
): TraegerOfferingFields {
  return {
    name: fields.name,
    description: fields.description ?? null,
    daysOfWeekMode: fields.days_of_week_mode,
    availableDays: fields.available_days ?? [],
    includesHolidayCare: fields.includes_holiday_care,
    includesLunch: fields.includes_lunch,
    capacity: fields.capacity ?? null,
    priceCents: fields.price_cents ?? null,
    isRequired: fields.is_required,
    countsAsCare: fields.counts_as_care,
    selectionGroup: fields.selection_group ?? "",
    selectionRule: fields.selection_rule || "optional",
    sortOrder: fields.sort_order ?? 0,
    pickupTimes: fields.pickup_times ?? {},
  };
}

function offeringDraftFromBackend(
  draft: BackendOfferingDraft,
): TraegerOfferingDraft {
  return {
    fields: offeringFieldsFromBackend(draft.fields),
    prices: (draft.prices ?? []).map((price) => ({
      weekday: price.weekday ? price.weekday : null,
      daysPerWeek: price.days_per_week ?? null,
      monthlyCents: price.monthly_cents,
      siblingCents: price.sibling_cents ?? null,
      mealExtra: price.meal_extra,
    })),
    contractTemplate: draft.contract_template
      ? termsFromBackend({
          ...draft.contract_template,
          term_months: draft.contract_template.term_months ?? null,
        })
      : null,
  };
}

function offeringFromBackend(offering: BackendOffering): TraegerOffering {
  return {
    id: String(offering.id),
    traegerPhaseId: String(offering.traeger_phase_id),
    draft: offeringDraftFromBackend(offering.draft),
    releasedVersion: offering.released_version ?? null,
    released: offering.released
      ? offeringDraftFromBackend(offering.released)
      : null,
    withdrawnAt: offering.withdrawn_at ?? null,
    schools: (offering.schools ?? []).map((school) => ({
      schoolId: String(school.school_id),
      appliedVersion: school.applied_version ?? null,
      outcome: outcomeOf(school.outcome),
      errorCode: school.error_code || null,
    })),
  };
}

function phaseFromBackend(phase: BackendPhase): TraegerPhase {
  return {
    id: String(phase.id),
    draft: phaseFieldsFromBackend(phase.draft),
    allSchools: phase.all_schools,
    releasedVersion: phase.released_version ?? null,
    released: phase.released ? phaseFieldsFromBackend(phase.released) : null,
    withdrawnAt: phase.withdrawn_at ?? null,
    schools: (phase.schools ?? []).map((school) => ({
      schoolId: String(school.school_id),
      selected: school.selected,
      appliedVersion: school.applied_version ?? null,
      outcome: outcomeOf(school.outcome),
      errorCode: school.error_code || null,
      recordedAt: school.recorded_at ?? null,
    })),
    updatedAt: phase.updated_at,
  };
}

function phaseDetailFromBackend(phase: BackendPhase): TraegerPhaseDetail {
  return {
    ...phaseFromBackend(phase),
    offerings: (phase.offerings ?? []).map(offeringFromBackend),
  };
}

function fieldChangesFromBackend(
  changes: BackendFieldChange[] | null | undefined,
): TraegerFieldChange[] {
  return (changes ?? []).map((change) => ({
    field: change.field,
    state: change.state,
    traeger: change.traeger ?? null,
    school: change.school ?? null,
  }));
}

/** The body the backend reads; IDs travel as numbers. */
function phaseInputToBackend(input: TraegerPhaseInput) {
  return {
    name: input.name.trim(),
    kind: input.kind,
    service_start_date: input.serviceStartDate,
    service_end_date: input.serviceEndDate,
    enrollment_open_at: input.enrollmentOpenAt,
    enrollment_close_at: input.enrollmentCloseAt,
    audience: input.audience,
    care_overflow_mode: input.careOverflowMode,
    care_offering_selection_mode: input.careOfferingSelectionMode,
    all_schools: input.allSchools,
    school_ids: input.allSchools ? [] : input.schoolIds.map(Number),
  };
}

function offeringDraftToBackend(draft: TraegerOfferingDraft) {
  const { fields } = draft;
  return {
    fields: {
      name: fields.name.trim(),
      description: fields.description?.trim()
        ? fields.description.trim()
        : null,
      days_of_week_mode: fields.daysOfWeekMode,
      available_days: [...fields.availableDays],
      includes_holiday_care: fields.includesHolidayCare,
      includes_lunch: fields.includesLunch,
      capacity: fields.capacity,
      price_cents: fields.priceCents,
      is_required: fields.isRequired,
      counts_as_care: fields.countsAsCare,
      selection_group: fields.selectionGroup.trim(),
      selection_rule: fields.selectionRule,
      sort_order: fields.sortOrder,
      pickup_times: { ...fields.pickupTimes },
    },
    prices: draft.prices.map((price) => ({
      ...(price.weekday === null
        ? { days_per_week: price.daysPerWeek }
        : { weekday: price.weekday }),
      monthly_cents: price.monthlyCents,
      ...(price.siblingCents === null
        ? {}
        : { sibling_cents: price.siblingCents }),
      meal_extra: price.mealExtra,
    })),
    ...(draft.contractTemplate
      ? { contract_template: termsToBackend(draft.contractTemplate) }
      : {}),
  };
}

// --- Requests ----------------------------------------------------------------

const BASE = "/api/traeger/catalog";

function phasePath(id: string): string {
  return `${BASE}/phases/${encodeURIComponent(id)}`;
}

export async function fetchTraegerPhases(): Promise<TraegerPhase[]> {
  const phases = await traegerRequest<BackendPhase[] | null>(`${BASE}/phases`);
  return (phases ?? []).map(phaseFromBackend);
}

export async function fetchTraegerPhase(
  id: string,
): Promise<TraegerPhaseDetail> {
  return phaseDetailFromBackend(
    await traegerRequest<BackendPhase>(phasePath(id)),
  );
}

export async function createTraegerPhase(
  input: TraegerPhaseInput,
): Promise<TraegerPhaseDetail> {
  return phaseDetailFromBackend(
    await traegerRequest<BackendPhase>(`${BASE}/phases`, {
      method: "POST",
      body: phaseInputToBackend(input),
    }),
  );
}

export async function updateTraegerPhase(
  id: string,
  input: TraegerPhaseInput,
): Promise<TraegerPhaseDetail> {
  return phaseDetailFromBackend(
    await traegerRequest<BackendPhase>(phasePath(id), {
      method: "PUT",
      body: phaseInputToBackend(input),
    }),
  );
}

export async function withdrawTraegerPhase(
  id: string,
): Promise<TraegerPhaseDetail> {
  return phaseDetailFromBackend(
    await traegerRequest<BackendPhase>(`${phasePath(id)}/withdraw`, {
      method: "POST",
    }),
  );
}

export async function createTraegerOffering(
  phaseId: string,
  draft: TraegerOfferingDraft,
): Promise<TraegerOffering> {
  return offeringFromBackend(
    await traegerRequest<BackendOffering>(`${phasePath(phaseId)}/offerings`, {
      method: "POST",
      body: offeringDraftToBackend(draft),
    }),
  );
}

export async function updateTraegerOffering(
  id: string,
  draft: TraegerOfferingDraft,
): Promise<TraegerOffering> {
  return offeringFromBackend(
    await traegerRequest<BackendOffering>(
      `${BASE}/offerings/${encodeURIComponent(id)}`,
      { method: "PUT", body: offeringDraftToBackend(draft) },
    ),
  );
}

export async function withdrawTraegerOffering(
  id: string,
): Promise<TraegerOffering> {
  return offeringFromBackend(
    await traegerRequest<BackendOffering>(
      `${BASE}/offerings/${encodeURIComponent(id)}/withdraw`,
      { method: "POST" },
    ),
  );
}

export async function previewTraegerPhase(
  id: string,
): Promise<TraegerCatalogPreview> {
  const preview = await traegerRequest<BackendPreview>(
    `${phasePath(id)}/preview`,
    { method: "POST" },
  );
  return {
    traegerPhaseId: String(preview.traeger_phase_id),
    version: preview.version,
    fingerprint: preview.fingerprint,
    schools: (preview.schools ?? []).map((school) => ({
      schoolId: String(school.school_id),
      action: school.action,
      outcome: school.outcome,
      errorCode: school.error_code || null,
      phase: fieldChangesFromBackend(school.phase),
      offerings: (school.offerings ?? []).map((offering) => ({
        traegerOfferingId: String(offering.traeger_offering_id),
        name: offering.name,
        action: offering.action,
        fields: fieldChangesFromBackend(offering.fields),
        pricesFrom: offering.prices_from ?? null,
        contractTemplate: offering.contract_template,
      })),
    })),
  };
}

export async function rolloutTraegerPhase(
  id: string,
  fingerprint: string,
): Promise<TraegerRollout> {
  const rollout = await traegerRequest<BackendRollout>(
    `${phasePath(id)}/rollout`,
    { method: "POST", body: { fingerprint } },
  );
  return {
    traegerPhaseId: String(rollout.traeger_phase_id),
    version: rollout.version,
    schools: (rollout.schools ?? []).map((school) => ({
      schoolId: String(school.school_id),
      outcome: school.outcome,
      errorCode: school.error_code || null,
    })),
  };
}

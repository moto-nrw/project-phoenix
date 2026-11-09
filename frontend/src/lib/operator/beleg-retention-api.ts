import { operatorFetch } from "./api-helpers";

/** Belegarten des Trägers (#4720, ADR 0184), in Anzeigereihenfolge. */
export const BELEG_TYPES = [
  "holiday_document",
  "care_invoice",
  "care_notice",
  "care_contract",
  "sepa_mandate",
  "sepa_debit",
] as const;

export type BelegType = (typeof BELEG_TYPES)[number];

const BELEG_TYPE_LABELS: Record<BelegType, string> = {
  holiday_document: "Ferienrechnungen, Stornos und Gutschriften",
  care_invoice: "Monatsrechnungen mit Storno",
  care_notice: "Jahresbescheide",
  care_contract: "Betreuungsverträge",
  sepa_mandate: "SEPA-Mandate",
  sepa_debit: "SEPA-Lastschriften",
};

export const TRIGGER_LABELS: Record<string, string> = {
  issued: "Ausstellung",
  contract_concluded: "Vertragsabschluss",
  contract_ended: "Vertragsende",
  last_collection: "Letzte eingezogene Lastschrift",
};

/** Ereignisse, die eine Weisung je Belegart nennen darf. */
export const TRIGGERS_BY_TYPE: Record<BelegType, readonly string[]> = {
  holiday_document: ["issued"],
  care_invoice: ["issued"],
  care_notice: ["issued"],
  care_contract: ["contract_ended", "contract_concluded"],
  sepa_mandate: ["last_collection"],
  sepa_debit: ["issued"],
};

/** Grundlagen einer Weisung (Löschkonzept Abschnitt 2). */
export const BASIS_LABELS: Record<string, string> = {
  booking_voucher: "Buchungsbeleg (§ 147 AO, § 257 HGB, § 14b UStG)",
  business_letter: "Handels- oder Geschäftsbrief",
  collection_agreement: "Inkassovereinbarung",
  not_applicable: "Vorschriften nicht anwendbar",
};

/** Grundlagen, die eine Weisung je Belegart nennen darf. */
export const BASES_BY_TYPE: Record<BelegType, readonly string[]> = {
  holiday_document: ["booking_voucher", "not_applicable"],
  care_invoice: ["booking_voucher", "not_applicable"],
  care_notice: ["booking_voucher", "business_letter", "not_applicable"],
  care_contract: ["booking_voucher", "business_letter", "not_applicable"],
  sepa_mandate: ["collection_agreement"],
  sepa_debit: ["booking_voucher", "not_applicable"],
};

/** Kürzeste Frist je Grundlage; ein Mandat zählt Monate. */
export function minimumPeriod(type: string, basis: string): number {
  if (type === "sepa_mandate") return 14;
  if (basis === "booking_voucher") return 8;
  if (basis === "business_letter") return 6;
  return 1;
}

export const CLARIFICATION_REASON_LABELS: Record<string, string> = {
  no_instruction: "Keine Weisung",
  no_collection: "Mandat ohne Einzug",
  no_trigger_date: "Kein Datum für das Ereignis",
};

export function belegTypeLabel(type: string): string {
  return BELEG_TYPE_LABELS[type as BelegType] ?? type;
}

/** Ein SEPA-Mandat zählt Monate, alle anderen Belegarten Jahre. */
export function periodUnit(type: string): "Monate" | "Jahre" {
  return type === "sepa_mandate" ? "Monate" : "Jahre";
}

export interface RetentionInstruction {
  readonly id: string;
  readonly belegType: string;
  readonly triggerEvent: string;
  readonly period: number;
  readonly basis: string;
  readonly legalBasis: string;
  readonly basisJustification: string;
  readonly instructedBy: string;
  readonly instructedOn: string;
  readonly instructionReference: string;
  readonly recordedAt: string;
}

export interface RetentionSuspension {
  readonly id: string;
  readonly belegType: string;
  readonly belegId: string | null;
  readonly reason: string;
  readonly scope: string;
  readonly responsible: string;
  readonly reviewOn: string;
}

export interface BelegSummary {
  readonly belegType: string;
  readonly scheduled: number;
  readonly running: number;
  readonly clarification: number;
  readonly suspended: number;
  readonly nextEnd: string | null;
}

export interface BelegClarification {
  readonly belegType: string;
  readonly reason: string;
  readonly count: number;
  readonly overdue: number;
  readonly listedOn: string;
  readonly reviewOn: string;
  readonly responsible: string;
  readonly lastReview: BelegClarificationReview | null;
}

interface BelegClarificationReview {
  readonly note: string;
  readonly escalated: boolean;
  readonly escalationReason: string;
  readonly reviewedOn: string;
  readonly nextReviewOn: string;
}

export interface BelegDeletion {
  readonly belegType: string;
  readonly deletedOn: string;
  readonly count: number;
}

export interface BelegRetentionOverview {
  readonly instructions: readonly RetentionInstruction[];
  readonly suspensions: readonly RetentionSuspension[];
  readonly summary: readonly BelegSummary[];
  readonly clarifications: readonly BelegClarification[];
  readonly deletions: readonly BelegDeletion[];
  readonly evaluatedOn: string | null;
}

export interface RecordInstructionRequest {
  readonly beleg_type: string;
  readonly trigger_event: string;
  readonly period: number;
  readonly basis: string;
  readonly legal_basis: string;
  readonly basis_justification: string;
  readonly instructed_by: string;
  readonly instructed_on: string;
  readonly instruction_reference: string;
}

export interface RecordSuspensionRequest {
  readonly beleg_type: string;
  readonly reason: string;
  readonly scope: string;
  readonly responsible: string;
  readonly review_on: string;
}

export interface RecordClarificationReviewRequest {
  readonly beleg_type: string;
  readonly reason: string;
  readonly note: string;
  readonly escalated: boolean;
  readonly escalation_reason: string;
  readonly next_review_on: string;
}

type Wire = Record<string, unknown>;

const text = (value: unknown): string =>
  typeof value === "string" ? value : "";
const num = (value: unknown): number => (typeof value === "number" ? value : 0);
const list = (value: unknown): Wire[] =>
  Array.isArray(value)
    ? value.filter((v): v is Wire => typeof v === "object" && v !== null)
    : [];
const id = (value: unknown): string =>
  typeof value === "number" || typeof value === "string" ? String(value) : "";

export function mapBelegRetentionOverview(
  data: unknown,
): BelegRetentionOverview {
  const wire = (typeof data === "object" && data !== null ? data : {}) as Wire;
  return {
    instructions: list(wire.instructions).map((row) => ({
      id: id(row.id),
      belegType: text(row.beleg_type),
      triggerEvent: text(row.trigger_event),
      period: num(row.period),
      basis: text(row.basis),
      legalBasis: text(row.legal_basis),
      basisJustification: text(row.basis_justification),
      instructedBy: text(row.instructed_by),
      instructedOn: text(row.instructed_on),
      instructionReference: text(row.instruction_reference),
      recordedAt: text(row.recorded_at),
    })),
    suspensions: list(wire.suspensions).map(mapSuspension),
    summary: list(wire.summary).map((row) => ({
      belegType: text(row.beleg_type),
      scheduled: num(row.scheduled),
      running: num(row.running),
      clarification: num(row.clarification),
      suspended: num(row.suspended),
      nextEnd: text(row.next_end) || null,
    })),
    clarifications: list(wire.clarifications).map((row) => ({
      belegType: text(row.beleg_type),
      reason: text(row.reason),
      count: num(row.count),
      overdue: num(row.overdue),
      listedOn: text(row.listed_on),
      reviewOn: text(row.review_on),
      responsible: text(row.responsible),
      lastReview: mapReview(row.last_review),
    })),
    deletions: list(wire.deletions).map((row) => ({
      belegType: text(row.beleg_type),
      deletedOn: text(row.deleted_on),
      count: num(row.count),
    })),
    evaluatedOn: text(wire.evaluated_on) || null,
  };
}

function mapReview(value: unknown): BelegClarificationReview | null {
  if (typeof value !== "object" || value === null) return null;
  const row = value as Wire;
  return {
    note: text(row.note),
    escalated: row.escalated === true,
    escalationReason: text(row.escalation_reason),
    reviewedOn: text(row.reviewed_on),
    nextReviewOn: text(row.next_review_on),
  };
}

function mapSuspension(row: Wire): RetentionSuspension {
  return {
    id: id(row.id),
    belegType: text(row.beleg_type),
    belegId: row.beleg_id ? id(row.beleg_id) : null,
    reason: text(row.reason),
    scope: text(row.scope),
    responsible: text(row.responsible),
    reviewOn: text(row.review_on),
  };
}

const base = (orgId: string) =>
  `/api/operator/provisioning/organizations/${encodeURIComponent(orgId)}/beleg-retention`;

export const belegRetentionService = {
  async getOverview(orgId: string): Promise<BelegRetentionOverview> {
    return mapBelegRetentionOverview(await operatorFetch<unknown>(base(orgId)));
  },
  async recordInstruction(
    orgId: string,
    data: RecordInstructionRequest,
  ): Promise<void> {
    await operatorFetch<unknown>(`${base(orgId)}/instructions`, {
      method: "POST",
      body: data,
    });
  },
  async recordSuspension(
    orgId: string,
    data: RecordSuspensionRequest,
  ): Promise<void> {
    await operatorFetch<unknown>(`${base(orgId)}/suspensions`, {
      method: "POST",
      body: data,
    });
  },
  async recordClarificationReview(
    orgId: string,
    data: RecordClarificationReviewRequest,
  ): Promise<void> {
    await operatorFetch<unknown>(`${base(orgId)}/clarification-reviews`, {
      method: "POST",
      body: data,
    });
  },
  async liftSuspension(orgId: string, suspensionId: string): Promise<void> {
    await operatorFetch<unknown>(
      `${base(orgId)}/suspensions/${encodeURIComponent(suspensionId)}`,
      { method: "DELETE" },
    );
  },
};

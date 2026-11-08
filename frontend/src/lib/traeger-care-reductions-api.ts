/**
 * Client of the rule chain of the monthly contribution (#4685): the care
 * rules of the Träger (income levels, siblings, single parents, cap) and
 * the evidences of its contracts. Money stays whole cent, shares basis
 * points (10000 = 100 %). Backend int64 IDs leave here as strings; calendar
 * dates stay "YYYY-MM-DD".
 */

import { traegerRequest } from "./traeger-api";
import type { CareModule } from "./traeger-care-billing-api";

/** Kinds of a care rule, in the order of the chain. */
export type CareRuleKind = "income_level" | "sibling" | "single_parent" | "cap";
export type CareEvidenceKind = "income_level" | "single_parent";

export const CARE_RULE_KINDS: readonly CareRuleKind[] = [
  "income_level",
  "sibling",
  "single_parent",
  "cap",
];

export interface CareRule {
  readonly id: string;
  readonly kind: CareRuleKind;
  readonly name: string;
  readonly validFrom: string;
  readonly validUntil: string | null;
  readonly schoolIds: readonly string[];
  /** Empty: every module of the schools. */
  readonly careOfferingIds: readonly string[];
  /** Empty: the schools of the rule. */
  readonly countSchoolIds: readonly string[];
  readonly siblingFromChild: number | null;
  readonly discountBasisPoints: number | null;
  readonly discountCents: number | null;
  readonly priceCents: number | null;
  readonly capCents: number | null;
  /**
   * A sibling rule charges every further child the sibling prices of the
   * price list (#6196) instead of a share, an amount or one price.
   */
  readonly tariffSiblingPrice?: boolean;
}

export type CareRuleInput = Omit<CareRule, "id">;

export interface CareRuleSet {
  readonly rules: readonly CareRule[];
  readonly schools: readonly { readonly id: string; readonly name: string }[];
  readonly modules: readonly CareModule[];
}

export interface CareEvidence {
  readonly id: string;
  readonly contractId: string;
  readonly kind: CareEvidenceKind;
  readonly ruleId: string | null;
  readonly validFrom: string;
  readonly validUntil: string | null;
  readonly evidence: string;
  readonly checkedOn: string;
}

export type CareEvidenceInput = Omit<CareEvidence, "id">;

export interface CareEvidenceContract {
  readonly contractId: string;
  readonly schoolId: string;
  readonly schoolName: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly evidences: readonly CareEvidence[];
}

export interface CareEvidenceSet {
  readonly contracts: readonly CareEvidenceContract[];
  readonly levels: readonly CareRule[];
}

interface BackendRule {
  id: number;
  kind: CareRuleKind;
  name: string;
  valid_from: string;
  valid_until: string | null;
  school_ids: number[] | null;
  care_offering_ids: number[] | null;
  count_school_ids: number[] | null;
  sibling_from_child: number | null;
  discount_basis_points: number | null;
  discount_cents: number | null;
  price_cents: number | null;
  cap_cents: number | null;
  tariff_sibling_price?: boolean;
}

interface BackendModule {
  school_id: number;
  school_name: string;
  care_offering_id: number;
  module_name: string;
  ends_at: string;
  booked_weekdays: CareModule["bookedWeekdays"] | null;
  booked_days_per_week?: number[] | null;
  contract_count: number;
}

interface BackendRuleSet {
  rules: BackendRule[] | null;
  schools: { id: number; name: string }[] | null;
  modules: BackendModule[] | null;
}

interface BackendEvidence {
  id: number;
  contract_id: number;
  kind: CareEvidenceKind;
  rule_id: number | null;
  valid_from: string;
  valid_until: string | null;
  evidence: string;
  checked_on: string;
}

interface BackendEvidenceSet {
  contracts:
    | {
        contract_id: number;
        school_id: number;
        school_name: string;
        child_first_name: string;
        child_last_name: string;
        evidences: BackendEvidence[] | null;
      }[]
    | null;
  levels: BackendRule[] | null;
}

const RULES_PATH = "/api/traeger/care-rules";
const EVIDENCES_PATH = "/api/traeger/care-evidences";

const ids = (values: number[] | null): string[] => (values ?? []).map(String);
const numbers = (values: readonly string[]): number[] => values.map(Number);

function ruleFromBackend(rule: BackendRule): CareRule {
  return {
    id: String(rule.id),
    kind: rule.kind,
    name: rule.name,
    validFrom: rule.valid_from,
    validUntil: rule.valid_until,
    schoolIds: ids(rule.school_ids),
    careOfferingIds: ids(rule.care_offering_ids),
    countSchoolIds: ids(rule.count_school_ids),
    siblingFromChild: rule.sibling_from_child,
    discountBasisPoints: rule.discount_basis_points,
    discountCents: rule.discount_cents,
    priceCents: rule.price_cents,
    capCents: rule.cap_cents,
    tariffSiblingPrice: rule.tariff_sibling_price ?? false,
  };
}

function ruleSetFromBackend(body: BackendRuleSet): CareRuleSet {
  return {
    rules: (body.rules ?? []).map(ruleFromBackend),
    schools: (body.schools ?? []).map((school) => ({
      id: String(school.id),
      name: school.name,
    })),
    modules: (body.modules ?? []).map((module) => ({
      schoolId: String(module.school_id),
      schoolName: module.school_name,
      careOfferingId: String(module.care_offering_id),
      moduleName: module.module_name,
      endsAt: module.ends_at,
      bookedWeekdays: module.booked_weekdays ?? [],
      bookedDaysPerWeek: module.booked_days_per_week ?? [],
      contractCount: module.contract_count,
    })),
  };
}

function evidenceFromBackend(evidence: BackendEvidence): CareEvidence {
  return {
    id: String(evidence.id),
    contractId: String(evidence.contract_id),
    kind: evidence.kind,
    ruleId: evidence.rule_id === null ? null : String(evidence.rule_id),
    validFrom: evidence.valid_from,
    validUntil: evidence.valid_until,
    evidence: evidence.evidence,
    checkedOn: evidence.checked_on,
  };
}

function evidenceSetFromBackend(body: BackendEvidenceSet): CareEvidenceSet {
  return {
    contracts: (body.contracts ?? []).map((contract) => ({
      contractId: String(contract.contract_id),
      schoolId: String(contract.school_id),
      schoolName: contract.school_name,
      childFirstName: contract.child_first_name,
      childLastName: contract.child_last_name,
      evidences: (contract.evidences ?? []).map(evidenceFromBackend),
    })),
    levels: (body.levels ?? []).map(ruleFromBackend),
  };
}

function ruleBody(input: CareRuleInput) {
  return {
    kind: input.kind,
    name: input.name,
    valid_from: input.validFrom,
    valid_until: input.validUntil,
    school_ids: numbers(input.schoolIds),
    care_offering_ids: numbers(input.careOfferingIds),
    count_school_ids: numbers(input.countSchoolIds),
    sibling_from_child: input.siblingFromChild,
    discount_basis_points: input.discountBasisPoints,
    discount_cents: input.discountCents,
    price_cents: input.priceCents,
    cap_cents: input.capCents,
    tariff_sibling_price: input.tariffSiblingPrice ?? false,
  };
}

function evidenceBody(input: CareEvidenceInput) {
  return {
    contract_id: Number(input.contractId),
    kind: input.kind,
    rule_id: input.ruleId === null ? null : Number(input.ruleId),
    valid_from: input.validFrom,
    valid_until: input.validUntil,
    evidence: input.evidence,
    checked_on: input.checkedOn,
  };
}

export async function fetchCareRules(): Promise<CareRuleSet> {
  return ruleSetFromBackend(await traegerRequest<BackendRuleSet>(RULES_PATH));
}

/** Creates a rule without `ruleId`, changes that one otherwise. */
export async function saveCareRule(
  ruleId: string | null,
  input: CareRuleInput,
): Promise<CareRuleSet> {
  return ruleSetFromBackend(
    await traegerRequest<BackendRuleSet>(
      ruleId ? `${RULES_PATH}/${ruleId}` : RULES_PATH,
      { method: ruleId ? "PUT" : "POST", body: ruleBody(input) },
    ),
  );
}

export async function deleteCareRule(ruleId: string): Promise<CareRuleSet> {
  return ruleSetFromBackend(
    await traegerRequest<BackendRuleSet>(`${RULES_PATH}/${ruleId}`, {
      method: "DELETE",
    }),
  );
}

/** Reads the evidences; the backend records the read per contract. */
export async function fetchCareEvidences(): Promise<CareEvidenceSet> {
  return evidenceSetFromBackend(
    await traegerRequest<BackendEvidenceSet>(EVIDENCES_PATH),
  );
}

export async function saveCareEvidence(
  evidenceId: string | null,
  input: CareEvidenceInput,
): Promise<CareEvidenceSet> {
  return evidenceSetFromBackend(
    await traegerRequest<BackendEvidenceSet>(
      evidenceId ? `${EVIDENCES_PATH}/${evidenceId}` : EVIDENCES_PATH,
      { method: evidenceId ? "PUT" : "POST", body: evidenceBody(input) },
    ),
  );
}

export async function deleteCareEvidence(
  evidenceId: string,
): Promise<CareEvidenceSet> {
  return evidenceSetFromBackend(
    await traegerRequest<BackendEvidenceSet>(
      `${EVIDENCES_PATH}/${evidenceId}`,
      { method: "DELETE" },
    ),
  );
}

/**
 * Client of the Elternabrechnung in the Träger portal (#4664): the
 * contribution rules of the Träger, the holiday contribution of a
 * registration and its confirmation, which freezes the contribution. Money
 * stays whole cent, shares stay basis points (10000 = 100 %). Backend int64
 * IDs leave here as strings.
 */

import {
  type BackendContribution,
  contributionFromBackend,
  type HolidayContribution,
} from "./holiday-contribution";
import { traegerRequest } from "./traeger-api";

export type {
  ContributionLine,
  HolidayContribution,
} from "./holiday-contribution";

export type ContributionRuleKind = "sibling" | "benefits_exemption";

export interface ContributionRule {
  readonly id: string;
  readonly kind: ContributionRuleKind;
  /** Calendar dates "YYYY-MM-DD"; null is open. */
  readonly validFrom: string;
  readonly validUntil: string | null;
  /** Sibling rule: from which child on (2 or 3). */
  readonly siblingFromChild: number | null;
  readonly discountBasisPoints: number | null;
  readonly discountCents: number | null;
  /** Exemption: the note the contribution shows. */
  readonly note: string;
}

export type ContributionRuleInput = Omit<ContributionRule, "id">;

interface BackendRule {
  id: number;
  kind: ContributionRuleKind;
  valid_from: string;
  valid_until: string | null;
  sibling_from_child: number | null;
  discount_basis_points: number | null;
  discount_cents: number | null;
  note: string;
}

function ruleFromBackend(rule: BackendRule): ContributionRule {
  return {
    id: String(rule.id),
    kind: rule.kind,
    validFrom: rule.valid_from,
    validUntil: rule.valid_until,
    siblingFromChild: rule.sibling_from_child,
    discountBasisPoints: rule.discount_basis_points,
    discountCents: rule.discount_cents,
    note: rule.note,
  };
}

function ruleToBackend(rule: ContributionRuleInput) {
  return {
    service: "holiday",
    kind: rule.kind,
    valid_from: rule.validFrom,
    valid_until: rule.validUntil,
    sibling_from_child: rule.siblingFromChild,
    discount_basis_points: rule.discountBasisPoints,
    discount_cents: rule.discountCents,
    note: rule.note,
  };
}

const RULES_PATH = "/api/traeger/contribution-rules";

/** The contribution rules of the Träger by kind and start. */
export async function fetchContributionRules(): Promise<ContributionRule[]> {
  const rules = await traegerRequest<BackendRule[] | null>(RULES_PATH);
  return (rules ?? []).map(ruleFromBackend);
}

export async function createContributionRule(
  rule: ContributionRuleInput,
): Promise<ContributionRule> {
  return ruleFromBackend(
    await traegerRequest<BackendRule>(RULES_PATH, {
      method: "POST",
      body: ruleToBackend(rule),
    }),
  );
}

export async function updateContributionRule(
  id: string,
  rule: ContributionRuleInput,
): Promise<ContributionRule> {
  return ruleFromBackend(
    await traegerRequest<BackendRule>(
      `${RULES_PATH}/${encodeURIComponent(id)}`,
      { method: "PUT", body: ruleToBackend(rule) },
    ),
  );
}

export async function deleteContributionRule(id: string): Promise<void> {
  await traegerRequest<unknown>(`${RULES_PATH}/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

/**
 * A body that pays the contribution of a family in its place (#5627). One
 * that a frozen contribution or an invoice names is in use and can only be
 * switched off.
 */
export interface CostBearer {
  readonly id: string;
  readonly name: string;
  readonly address: string;
  readonly fileReference: string;
  readonly active: boolean;
  readonly inUse: boolean;
}

export type CostBearerInput = Omit<CostBearer, "id" | "inUse">;

interface BackendCostBearer {
  id: number;
  name: string;
  address: string;
  file_reference: string;
  active: boolean;
  in_use?: boolean;
}

function costBearerFromBackend(bearer: BackendCostBearer): CostBearer {
  return {
    id: String(bearer.id),
    name: bearer.name,
    address: bearer.address,
    fileReference: bearer.file_reference,
    active: bearer.active,
    inUse: bearer.in_use ?? false,
  };
}

const COST_BEARERS_PATH = "/api/traeger/cost-bearers";

/** The cost bearers of the Träger by name. */
export async function fetchCostBearers(): Promise<CostBearer[]> {
  const bearers = await traegerRequest<BackendCostBearer[] | null>(
    COST_BEARERS_PATH,
  );
  return (bearers ?? []).map(costBearerFromBackend);
}

/** Creates a cost bearer, or changes the one id names. */
export async function saveCostBearer(
  id: string | null,
  bearer: CostBearerInput,
): Promise<CostBearer> {
  return costBearerFromBackend(
    await traegerRequest<BackendCostBearer>(
      id ? `${COST_BEARERS_PATH}/${encodeURIComponent(id)}` : COST_BEARERS_PATH,
      {
        method: id ? "PUT" : "POST",
        body: {
          name: bearer.name,
          address: bearer.address,
          file_reference: bearer.fileReference,
          active: bearer.active,
        },
      },
    ),
  );
}

export async function deleteCostBearer(id: string): Promise<void> {
  await traegerRequest<unknown>(
    `${COST_BEARERS_PATH}/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

const registrationPath = (periodId: string, registrationId: string) =>
  `/api/traeger/holiday-periods/${encodeURIComponent(periodId)}/registrations/${encodeURIComponent(registrationId)}`;

/**
 * The frozen contribution of a registration, or what a confirmation would
 * freeze now: with the exemption the Träger-Büro is about to choose, or the
 * parents' statement when none is given.
 */
export async function fetchHolidayContribution(
  periodId: string,
  registrationId: string,
  benefitsExemption?: boolean,
  costBearerId?: string,
): Promise<HolidayContribution> {
  const params = new URLSearchParams();
  if (benefitsExemption !== undefined) {
    params.set("benefits_exemption", String(benefitsExemption));
  }
  if (costBearerId) params.set("cost_bearer_id", costBearerId);
  const query = params.size > 0 ? `?${params.toString()}` : "";
  return contributionFromBackend(
    await traegerRequest<BackendContribution>(
      `${registrationPath(periodId, registrationId)}/contribution${query}`,
    ),
  );
}

/**
 * Confirms the registration and freezes its contribution, with the debtor
 * number of the family (#5621); an empty one confirms without. A cost
 * bearer pays instead of the family (#5627).
 */
export async function confirmHolidayRegistration(
  periodId: string,
  registrationId: string,
  benefitsExemption: boolean,
  debtorNumber = "",
  costBearerId?: string,
): Promise<HolidayContribution> {
  const confirmation = await traegerRequest<{
    contribution: BackendContribution;
  }>(`${registrationPath(periodId, registrationId)}/confirm`, {
    method: "POST",
    body: {
      benefits_exemption: benefitsExemption,
      debtor_number: debtorNumber,
      ...(costBearerId ? { cost_bearer_id: Number(costBearerId) } : {}),
    },
  });
  return contributionFromBackend(confirmation.contribution);
}

/**
 * Cancels the registration (#5662). In a sibling group the Elternabrechnung
 * takes it out of the group and computes the contribution of the other
 * registrations again; the caller reads the registration again.
 */
export async function cancelHolidayRegistration(
  periodId: string,
  registrationId: string,
): Promise<void> {
  await traegerRequest<{ id: number; status: string }>(
    `${registrationPath(periodId, registrationId)}/cancel`,
    { method: "POST", body: {} },
  );
}

/**
 * Changes the debtor number of a confirmed registration until an invoice
 * bills it (#5621); returns the number the registration now has.
 */
export async function changeHolidayDebtorNumber(
  periodId: string,
  registrationId: string,
  debtorNumber: string,
): Promise<string> {
  const changed = await traegerRequest<{ debtor_number: string }>(
    `${registrationPath(periodId, registrationId)}/debtor-number`,
    { method: "PUT", body: { debtor_number: debtorNumber } },
  );
  return changed.debtor_number;
}

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
  ChildContribution,
  ContributionLine,
  ContributionLineKind,
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

const registrationPath = (periodId: string, registrationId: string) =>
  `/api/traeger/holiday-periods/${encodeURIComponent(periodId)}/registrations/${encodeURIComponent(registrationId)}`;

/** The frozen contribution of a registration, or what a confirmation would freeze now. */
export async function fetchHolidayContribution(
  periodId: string,
  registrationId: string,
): Promise<HolidayContribution> {
  return contributionFromBackend(
    await traegerRequest<BackendContribution>(
      `${registrationPath(periodId, registrationId)}/contribution`,
    ),
  );
}

/** Confirms the registration and freezes its contribution. */
export async function confirmHolidayRegistration(
  periodId: string,
  registrationId: string,
  benefitsExemption: boolean,
): Promise<HolidayContribution> {
  const confirmation = await traegerRequest<{
    contribution: BackendContribution;
  }>(`${registrationPath(periodId, registrationId)}/confirm`, {
    method: "POST",
    body: { benefits_exemption: benefitsExemption },
  });
  return contributionFromBackend(confirmation.contribution);
}

/**
 * Client of the children of the families (#5918): how many children a
 * family has for the sibling rule, siblings without a care contract
 * included. The parents state a number, the Abrechnung confirms or changes
 * it with a validity. A family is named by one of its contracts, never by
 * its address. Backend int64 IDs leave here as strings; calendar dates stay
 * "YYYY-MM-DD".
 */
import { traegerRequest } from "./traeger-api";
import type { CareRule } from "./traeger-care-reductions-api";

/** The state of a family's number today; the most urgent check wins. */
export type FamilyChildrenStatus =
  | "missing"
  | "stated"
  | "changed"
  | "confirmed"
  | "expiring"
  | "expired"
  | "upcoming";

export interface FamilyChildCount {
  readonly id: string;
  readonly contractId: string;
  readonly statedChildren: number | null;
  /** Instant of the parents' statement, RFC 3339. */
  readonly statedAt: string | null;
  readonly confirmedChildren: number | null;
  readonly validFrom: string | null;
  readonly validUntil: string | null;
  readonly confirmedAt: string | null;
}

interface FamilyContractChild {
  readonly contractId: string;
  readonly schoolName: string;
  readonly childFirstName: string;
  readonly childLastName: string;
}

export interface FamilyChildren {
  readonly recipientFirstName: string;
  readonly recipientLastName: string;
  readonly recipientEmail: string;
  readonly children: readonly FamilyContractChild[];
  readonly count: FamilyChildCount | null;
  readonly status: FamilyChildrenStatus;
}

export interface FamilyChildrenSet {
  readonly families: readonly FamilyChildren[];
  /** The sibling rules that count the family's number. */
  readonly rules: readonly Pick<CareRule, "id" | "name">[];
}

export interface FamilyChildrenInput {
  readonly contractId: string;
  readonly children: number;
  readonly validFrom: string;
  readonly validUntil: string | null;
}

interface BackendCount {
  id: number;
  contract_id: number;
  stated_children: number | null;
  stated_at: string | null;
  confirmed_children: number | null;
  valid_from: string | null;
  valid_until: string | null;
  confirmed_at: string | null;
}

interface BackendFamily {
  recipient_first_name: string;
  recipient_last_name: string;
  recipient_email: string;
  children:
    | {
        contract_id: number;
        school_name: string;
        child_first_name: string;
        child_last_name: string;
      }[]
    | null;
  count: BackendCount | null;
  status: FamilyChildrenStatus;
}

interface BackendSet {
  families: BackendFamily[] | null;
  rules: { id: number; name: string }[] | null;
}

const PATH = "/api/traeger/family-children";

function countFromBackend(count: BackendCount): FamilyChildCount {
  return {
    id: String(count.id),
    contractId: String(count.contract_id),
    statedChildren: count.stated_children,
    statedAt: count.stated_at,
    confirmedChildren: count.confirmed_children,
    validFrom: count.valid_from,
    validUntil: count.valid_until,
    confirmedAt: count.confirmed_at,
  };
}

function familyChildrenSetFromBackend(body: BackendSet): FamilyChildrenSet {
  return {
    families: (body.families ?? []).map((family) => ({
      recipientFirstName: family.recipient_first_name,
      recipientLastName: family.recipient_last_name,
      recipientEmail: family.recipient_email,
      children: (family.children ?? []).map((child) => ({
        contractId: String(child.contract_id),
        schoolName: child.school_name,
        childFirstName: child.child_first_name,
        childLastName: child.child_last_name,
      })),
      count: family.count ? countFromBackend(family.count) : null,
      status: family.status,
    })),
    rules: (body.rules ?? []).map((rule) => ({
      id: String(rule.id),
      name: rule.name,
    })),
  };
}

export async function fetchFamilyChildren(): Promise<FamilyChildrenSet> {
  return familyChildrenSetFromBackend(await traegerRequest<BackendSet>(PATH));
}

/** Confirms or changes the number of the family of a contract. */
export async function confirmFamilyChildren(
  input: FamilyChildrenInput,
): Promise<FamilyChildrenSet> {
  return familyChildrenSetFromBackend(
    await traegerRequest<BackendSet>(PATH, {
      method: "PUT",
      body: JSON.stringify({
        contract_id: Number(input.contractId),
        children: input.children,
        valid_from: input.validFrom,
        valid_until: input.validUntil,
      }),
    }),
  );
}

/** Removes the number of the family of a contract. */
export async function removeFamilyChildren(
  contractId: string,
): Promise<FamilyChildrenSet> {
  return familyChildrenSetFromBackend(
    await traegerRequest<BackendSet>(
      `${PATH}/${encodeURIComponent(contractId)}`,
      { method: "DELETE" },
    ),
  );
}

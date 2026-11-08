/**
 * Client of the cost bearer shares of the monthly contribution (#4687): which
 * cost bearer (Bildung und Teilhabe, Jugendamt, another subsidy) pays which
 * part of a contract's contribution, with its validity and a reference to the
 * proof, and the list of a run for one cost bearer. Shares stay basis points
 * (10000 = 100 %) or whole cent; backend int64 IDs leave here as strings;
 * calendar dates stay "YYYY-MM-DD".
 */

import { traegerRequest } from "./traeger-api";
import { type BillingFile, billingFile } from "./traeger-billing-api";
import type { CareCostShareKind } from "./traeger-care-billing-api";

/** Where a share stands today; `expiring` ends within `warningDays`. */
export type CareCostShareStatus =
  "upcoming" | "active" | "expiring" | "expired";

export interface CareCostShare {
  readonly id: string;
  readonly contractId: string;
  readonly kind: CareCostShareKind;
  readonly costBearerId: string;
  readonly costBearerName: string;
  readonly shareBasisPoints: number | null;
  readonly shareCents: number | null;
  readonly validFrom: string;
  readonly validUntil: string;
  readonly reference: string;
  readonly checkedOn: string;
  readonly status: CareCostShareStatus;
}

export type CareCostShareInput = Omit<
  CareCostShare,
  "id" | "costBearerName" | "status"
>;

export interface CareCostShareContract {
  readonly contractId: string;
  readonly schoolId: string;
  readonly schoolName: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly shares: readonly CareCostShare[];
}

export interface CareCostShareBearer {
  readonly id: string;
  readonly name: string;
  readonly active: boolean;
}

export interface CareCostShareSet {
  readonly contracts: readonly CareCostShareContract[];
  readonly costBearers: readonly CareCostShareBearer[];
  /** Shares that end soon without a follow-up. */
  readonly expiring: number;
  readonly warningDays: number;
}

export type CareCostShareExportFormat = "pdf" | "xlsx" | "csv";

interface BackendShare {
  id: number;
  contract_id: number;
  kind: CareCostShareKind;
  cost_bearer_id: number;
  cost_bearer_name: string;
  share_basis_points: number | null;
  share_cents: number | null;
  valid_from: string;
  valid_until: string;
  reference: string;
  checked_on: string;
  status: CareCostShareStatus;
}

interface BackendSet {
  contracts:
    | {
        contract_id: number;
        school_id: number;
        school_name: string;
        child_first_name: string;
        child_last_name: string;
        shares: BackendShare[] | null;
      }[]
    | null;
  cost_bearers: { id: number; name: string; active: boolean }[] | null;
  expiring: number;
  warning_days: number;
}

const SHARES_PATH = "/api/traeger/care-cost-shares";

function shareFromBackend(share: BackendShare): CareCostShare {
  return {
    id: String(share.id),
    contractId: String(share.contract_id),
    kind: share.kind,
    costBearerId: String(share.cost_bearer_id),
    costBearerName: share.cost_bearer_name,
    shareBasisPoints: share.share_basis_points,
    shareCents: share.share_cents,
    validFrom: share.valid_from,
    validUntil: share.valid_until,
    reference: share.reference,
    checkedOn: share.checked_on,
    status: share.status,
  };
}

export function costShareSetFromBackend(body: BackendSet): CareCostShareSet {
  return {
    contracts: (body.contracts ?? []).map((contract) => ({
      contractId: String(contract.contract_id),
      schoolId: String(contract.school_id),
      schoolName: contract.school_name,
      childFirstName: contract.child_first_name,
      childLastName: contract.child_last_name,
      shares: (contract.shares ?? []).map(shareFromBackend),
    })),
    costBearers: (body.cost_bearers ?? []).map((bearer) => ({
      id: String(bearer.id),
      name: bearer.name,
      active: bearer.active,
    })),
    expiring: body.expiring,
    warningDays: body.warning_days,
  };
}

function shareBody(input: CareCostShareInput) {
  return {
    contract_id: Number(input.contractId),
    kind: input.kind,
    cost_bearer_id: Number(input.costBearerId),
    share_basis_points: input.shareBasisPoints,
    share_cents: input.shareCents,
    valid_from: input.validFrom,
    valid_until: input.validUntil,
    reference: input.reference,
    checked_on: input.checkedOn,
  };
}

/** Reads the shares; the backend records the read per contract. */
export async function fetchCareCostShares(): Promise<CareCostShareSet> {
  return costShareSetFromBackend(await traegerRequest<BackendSet>(SHARES_PATH));
}

/** Creates a share without `shareId`, changes that one otherwise. */
export async function saveCareCostShare(
  shareId: string | null,
  input: CareCostShareInput,
): Promise<CareCostShareSet> {
  return costShareSetFromBackend(
    await traegerRequest<BackendSet>(
      shareId ? `${SHARES_PATH}/${encodeURIComponent(shareId)}` : SHARES_PATH,
      { method: shareId ? "PUT" : "POST", body: shareBody(input) },
    ),
  );
}

export async function deleteCareCostShare(
  shareId: string,
): Promise<CareCostShareSet> {
  return costShareSetFromBackend(
    await traegerRequest<BackendSet>(
      `${SHARES_PATH}/${encodeURIComponent(shareId)}`,
      { method: "DELETE" },
    ),
  );
}

/** The list of a run for one cost bearer, to bill its part separately. */
export async function downloadCareCostShareList(
  runId: string,
  costBearerId: string,
  format: CareCostShareExportFormat,
): Promise<BillingFile> {
  return billingFile(
    `/api/traeger/care-billing-runs/${encodeURIComponent(runId)}/cost-shares/${encodeURIComponent(costBearerId)}/export?format=${format}`,
    {},
    `kostentraeger.${format}`,
  );
}

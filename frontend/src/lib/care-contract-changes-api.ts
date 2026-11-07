/**
 * The Träger-Büro's client of the parents' changes of the booked care
 * (#5979): the open change of a contract's child and its decision.
 */

import {
  type BackendContractChanges,
  type ContractChangeDecision,
  type ContractChanges,
  contractChangesFromBackend,
  decisionBody,
} from "./care-contract-changes";
import { traegerRequest } from "./traeger-api";

const changesPath = (contractId: string) =>
  `/api/traeger/contract-changes/${encodeURIComponent(contractId)}`;

export async function fetchContractChanges(
  contractId: string,
  effectiveFrom?: string,
): Promise<ContractChanges> {
  return contractChangesFromBackend(
    await traegerRequest<BackendContractChanges>(
      `${changesPath(contractId)}${effectiveFrom ? `?effective_from=${encodeURIComponent(effectiveFrom)}` : ""}`,
    ),
  );
}

/** Approves or rejects one request; answers the changes afterwards. */
export async function decideContractChange(
  contractId: string,
  requestId: string,
  decision: ContractChangeDecision,
): Promise<ContractChanges> {
  return contractChangesFromBackend(
    await traegerRequest<BackendContractChanges>(
      `${changesPath(contractId)}/requests/${encodeURIComponent(requestId)}/decide`,
      { method: "POST", body: decisionBody(decision) },
    ),
  );
}

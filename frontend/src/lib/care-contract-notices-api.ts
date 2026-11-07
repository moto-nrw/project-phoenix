/**
 * The Träger-Büro's client of withdrawal and termination (#4682): what a
 * contract allows today, and a declaration the office received by mail or
 * letter.
 */

import {
  type BackendNoticeOptions,
  type ContractNoticeOptions,
  declarationBody,
  type NoticeDeclaration,
  noticeOptionsFromBackend,
} from "./care-contract-notices";
import { traegerRequest } from "./traeger-api";

// ── Träger-Büro ────────────────────────────────────────────────────────────

const officePath = (id: string) =>
  `/api/traeger/contract-notices/${encodeURIComponent(id)}`;

export async function fetchContractNoticeOptions(
  id: string,
  receivedOn?: string,
): Promise<ContractNoticeOptions> {
  return noticeOptionsFromBackend(
    await traegerRequest<BackendNoticeOptions>(
      receivedOn
        ? `${officePath(id)}?${new URLSearchParams({ received_on: receivedOn })}`
        : officePath(id),
    ),
  );
}

/** Records a declaration the office received by mail or letter on a day. */
export async function recordContractNotice(
  id: string,
  declaration: NoticeDeclaration & {
    readonly receivedOn: string;
    readonly dispatchedOn?: string;
  },
): Promise<ContractNoticeOptions> {
  return noticeOptionsFromBackend(
    await traegerRequest<BackendNoticeOptions>(officePath(id), {
      method: "POST",
      body: {
        ...declarationBody(declaration),
        received_on: declaration.receivedOn,
        ...(declaration.kind === "widerruf"
          ? { dispatched_on: declaration.dispatchedOn }
          : {}),
      },
    }),
  );
}

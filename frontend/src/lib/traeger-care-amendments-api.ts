/**
 * Client of the changes of a single invoice in the review of a monthly run
 * (#6197): amount, recipient, address and positions, each with a reason,
 * and the revocation of a change. Only the page of one invoice loads it.
 */

import { traegerRequest } from "./traeger-api";
import type { PostalAddress } from "./traeger-billing-terms-api";
import {
  type BackendRunDetail,
  type CareAmendmentKind,
  type CareRunDetail,
  detailFromBackend,
} from "./traeger-care-billing-api";

/** One change of one invoice, made on the review with `fingerprint`. */
export type CareAmendmentInput = {
  readonly contractId: string;
  readonly service: "care" | "meal";
  readonly fingerprint: string;
  readonly reason: string;
} & (
  | { readonly kind: "amount"; readonly totalCents: number }
  | {
      readonly kind: "add_position";
      readonly label: string;
      readonly amountCents: number;
    }
  | { readonly kind: "remove_position"; readonly positionKey: string }
  | {
      readonly kind: "recipient";
      readonly recipient: {
        readonly firstName: string;
        readonly lastName: string;
        readonly secondName: string;
        readonly email: string;
      };
    }
  | { readonly kind: "address"; readonly address: PostalAddress }
  | {
      readonly kind: "revoke";
      readonly revokesId: string;
      readonly revokesCarried: boolean;
    }
);

function amendmentBody(input: CareAmendmentInput): Record<string, unknown> {
  const body: Record<string, unknown> = {
    contract_id: Number(input.contractId),
    service: input.service,
    kind: input.kind satisfies CareAmendmentKind | "revoke",
    fingerprint: input.fingerprint,
    reason: input.reason,
  };
  switch (input.kind) {
    case "amount":
      return { ...body, total_cents: input.totalCents };
    case "add_position":
      return { ...body, label: input.label, amount_cents: input.amountCents };
    case "remove_position":
      return { ...body, position_key: input.positionKey };
    case "recipient":
      return {
        ...body,
        recipient: {
          first_name: input.recipient.firstName,
          last_name: input.recipient.lastName,
          second_name: input.recipient.secondName,
          email: input.recipient.email,
        },
      };
    case "address":
      return {
        ...body,
        address: {
          street: input.address.street,
          postal_code: input.address.postalCode,
          city: input.address.city,
        },
      };
    case "revoke":
      return {
        ...body,
        revokes_id: Number(input.revokesId),
        revokes_carried: input.revokesCarried,
      };
  }
}

/** Appends a change of one invoice; answers the run after it. */
export async function amendCareInvoice(
  runId: string,
  input: CareAmendmentInput,
): Promise<CareRunDetail> {
  return detailFromBackend(
    await traegerRequest<BackendRunDetail>(
      `/api/traeger/care-billing-runs/${encodeURIComponent(runId)}/amendments`,
      { method: "POST", body: amendmentBody(input) },
    ),
  );
}

/**
 * Client of the SEPA direct debit (#4695): the creditor of the Träger
 * (Gläubiger-ID, pre-notification days, what an invoice without a mandate
 * does) and the mandates of the families. The IBAN of a mandate never comes
 * back: the backend answers its mask only.
 * Backend int64 IDs leave here as strings; calendar dates stay "YYYY-MM-DD".
 */

import { traegerRequest } from "./traeger-api";

export type MissingMandate = "transfer" | "block";
export type MandateKind = "recurring" | "one_off";
export type MandateSequence = "first" | "recurring";
export type MandateSource = "office" | "import" | "parent";
export type MandateAction =
  "create" | "update" | "revoke" | "read" | "reveal" | "collect";

export const MIN_PRENOTIFICATION_DAYS = 1;
export const MAX_PRENOTIFICATION_DAYS = 30;

export interface SepaCreditor {
  /** Gläubiger-Identifikationsnummer; "" until the Träger sets one. */
  readonly creditorId: string;
  readonly prenotificationDays: number;
  readonly missingMandate: MissingMandate;
  readonly updatedAt: string | null;
}

export interface SepaMandate {
  readonly id: string;
  readonly contractId: string;
  readonly familyEmail: string;
  readonly partyFirstName: string;
  readonly partyLastName: string;
  readonly reference: string;
  /** Calendar date of the signature, "YYYY-MM-DD". */
  readonly signedOn: string;
  readonly kind: MandateKind;
  readonly sequence: MandateSequence;
  readonly accountHolder: string;
  /** The creditor identifier the mandate was given to. */
  readonly creditorId: string;
  /** False after the Träger changed its creditor identifier: a new mandate is needed. */
  readonly creditorCurrent: boolean;
  /** The IBAN with only country and last four characters visible. */
  readonly ibanMasked: string;
  readonly bic: string;
  readonly source: MandateSource;
  /** Calendar date the revocation arrived; null while in force. */
  readonly revokedOn: string | null;
  /** Day of the last collection moto counted (#4697); null before one. */
  readonly lastCollectedOn: string | null;
  /** First day the mandate no longer holds: 36 months without a collection (#4697). */
  readonly expiresOn: string | null;
  readonly createdAt: string;
}

export interface SepaMandateInput {
  readonly contractId: string;
  /** "" lets moto form one. */
  readonly reference: string;
  readonly signedOn: string;
  readonly kind: MandateKind;
  readonly sequence: MandateSequence;
  readonly accountHolder: string;
  readonly iban: string;
  readonly bic: string;
}

export interface SepaMandateEvent {
  readonly id: string;
  readonly action: MandateAction;
  readonly source: MandateSource | "run";
  /** The fields a change touched, never their values. */
  readonly fields: readonly string[];
  /** The account that acted; null once the account is deleted. */
  readonly accountId: string | null;
  readonly occurredAt: string;
}

interface BackendCreditor {
  creditor_id: string;
  prenotification_days: number;
  missing_mandate: MissingMandate;
  updated_at: string | null;
}

interface BackendMandate {
  id: number;
  contract_id: number;
  family_email: string;
  party_first_name: string;
  party_last_name: string;
  reference: string;
  signed_on: string;
  kind: MandateKind;
  sequence: MandateSequence;
  account_holder: string;
  creditor_id: string;
  creditor_current: boolean;
  iban_masked: string;
  bic: string;
  source: MandateSource;
  revoked_on: string | null;
  last_collected_on?: string | null;
  expires_on?: string | null;
  created_at: string;
}

interface BackendEvent {
  id: number;
  action: MandateAction;
  source: MandateSource | "run";
  fields: string[] | null;
  account_id: number | null;
  occurred_at: string;
}

function creditorFromBackend(creditor: BackendCreditor): SepaCreditor {
  return {
    creditorId: creditor.creditor_id,
    prenotificationDays: creditor.prenotification_days,
    missingMandate: creditor.missing_mandate,
    updatedAt: creditor.updated_at,
  };
}

function mandateFromBackend(mandate: BackendMandate): SepaMandate {
  return {
    id: String(mandate.id),
    contractId: String(mandate.contract_id),
    familyEmail: mandate.family_email,
    partyFirstName: mandate.party_first_name,
    partyLastName: mandate.party_last_name,
    reference: mandate.reference,
    signedOn: mandate.signed_on,
    kind: mandate.kind,
    sequence: mandate.sequence,
    accountHolder: mandate.account_holder,
    creditorId: mandate.creditor_id,
    creditorCurrent: mandate.creditor_current,
    ibanMasked: mandate.iban_masked,
    bic: mandate.bic,
    source: mandate.source,
    revokedOn: mandate.revoked_on,
    lastCollectedOn: mandate.last_collected_on ?? null,
    expiresOn: mandate.expires_on || null,
    createdAt: mandate.created_at,
  };
}

const CREDITOR_PATH = "/api/traeger/sepa-creditor";
const MANDATES_PATH = "/api/traeger/sepa-mandates";

/** The creditor of the Träger, the defaults where it set nothing. */
export async function fetchSepaCreditor(): Promise<SepaCreditor> {
  return creditorFromBackend(
    await traegerRequest<BackendCreditor>(CREDITOR_PATH),
  );
}

export async function saveSepaCreditor(
  creditor: Pick<
    SepaCreditor,
    "creditorId" | "prenotificationDays" | "missingMandate"
  >,
): Promise<SepaCreditor> {
  return creditorFromBackend(
    await traegerRequest<BackendCreditor>(CREDITOR_PATH, {
      method: "PUT",
      body: {
        creditor_id: creditor.creditorId.trim(),
        prenotification_days: creditor.prenotificationDays,
        missing_mandate: creditor.missingMandate,
      },
    }),
  );
}

/** The mandates of the Träger, of one contract's family when given. */
export async function fetchSepaMandates(
  contractId?: string,
): Promise<SepaMandate[]> {
  const query = contractId
    ? `?contract_id=${encodeURIComponent(contractId)}`
    : "";
  const mandates = await traegerRequest<BackendMandate[] | null>(
    `${MANDATES_PATH}${query}`,
  );
  return (mandates ?? []).map(mandateFromBackend);
}

/** The request body of a new mandate. */
function sepaMandateBody(input: SepaMandateInput) {
  return {
    contract_id: Number(input.contractId),
    reference: input.reference.trim(),
    signed_on: input.signedOn,
    kind: input.kind,
    sequence: input.kind === "one_off" ? "first" : input.sequence,
    account_holder: input.accountHolder.trim(),
    iban: input.iban,
    bic: input.bic.trim(),
  };
}

export async function createSepaMandate(
  input: SepaMandateInput,
): Promise<SepaMandate> {
  return mandateFromBackend(
    await traegerRequest<BackendMandate>(MANDATES_PATH, {
      method: "POST",
      body: sepaMandateBody(input),
    }),
  );
}

/** Ends a mandate on the day the revocation arrived. */
export async function revokeSepaMandate(
  id: string,
  revokedOn: string,
): Promise<SepaMandate> {
  return mandateFromBackend(
    await traegerRequest<BackendMandate>(
      `${MANDATES_PATH}/${encodeURIComponent(id)}/revoke`,
      { method: "POST", body: { revoked_on: revokedOn } },
    ),
  );
}

/** The trail of a mandate, oldest first. */
export async function fetchSepaMandateHistory(
  id: string,
): Promise<SepaMandateEvent[]> {
  const events = await traegerRequest<BackendEvent[] | null>(
    `${MANDATES_PATH}/${encodeURIComponent(id)}/history`,
  );
  return (events ?? []).map((event) => ({
    id: String(event.id),
    action: event.action,
    source: event.source,
    fields: event.fields ?? [],
    accountId: event.account_id === null ? null : String(event.account_id),
    occurredAt: event.occurred_at,
  }));
}

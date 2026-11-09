// Unterschrift eines Betreuungsvertrags (#4679): dieselbe Ansicht über den
// einmaligen Link aus der Mail (ohne Konto) und in der Eltern-App unter
// „Dokumente“. Das Backend liefert Vertrag, Prüfsumme des PDFs und
// Widerrufsbelehrung; die Unterschrift schließt den Vertrag ab.

import { apiErrorFromResponse, transportFetch } from "./api-error";
import type { SignatureValue } from "./signature";
import type { CareContractStatus, Weekday } from "./traeger-care-contracts-api";

export interface SigningService {
  readonly weekday: Weekday;
  readonly moduleName: string;
  readonly endsAt: string;
  readonly lastDay: string | null;
}

interface SigningSignature {
  readonly method: "drawn" | "typed" | "paper";
  readonly signerName: string;
  readonly signedAt: string;
  readonly earlyStartRequested: boolean;
  readonly withdrawalEndsOn: string | null;
}

export interface SigningView {
  readonly status: CareContractStatus;
  readonly traegerName: string;
  readonly partyName: string;
  readonly childName: string;
  readonly schoolName: string;
  readonly phaseName: string;
  readonly effectiveFrom: string;
  readonly endsOn: string | null;
  readonly priceCents: number | null;
  readonly services: readonly SigningService[];
  readonly documentSha256: string;
  readonly withdrawalNotice: readonly string[];
  readonly signature: SigningSignature | null;
}

interface BackendSigningView {
  status: CareContractStatus;
  traeger_name: string;
  party_name: string;
  version: {
    child_first_name: string;
    child_last_name: string;
    school_name: string;
    phase_name: string;
    effective_from: string;
    ends_on: string | null;
    price_cents: number | null;
    services: {
      weekday: Weekday;
      module_name: string;
      ends_at: string;
      last_day: string | null;
    }[];
  };
  document_sha256: string;
  withdrawal_notice: string[] | null;
  signature: {
    method: SigningSignature["method"];
    signer_name: string;
    signed_at: string;
    early_start_requested: boolean;
    withdrawal_ends_on: string | null;
  } | null;
}

function mapSigningView(raw: BackendSigningView): SigningView {
  const version = raw.version;
  return {
    status: raw.status,
    traegerName: raw.traeger_name,
    partyName: raw.party_name,
    childName: `${version.child_first_name} ${version.child_last_name}`.trim(),
    schoolName: version.school_name,
    phaseName: version.phase_name,
    effectiveFrom: version.effective_from,
    endsOn: version.ends_on,
    priceCents: version.price_cents,
    services: (version.services ?? []).map((service) => ({
      weekday: service.weekday,
      moduleName: service.module_name,
      endsAt: service.ends_at,
      lastDay: service.last_day,
    })),
    documentSha256: raw.document_sha256,
    withdrawalNotice: raw.withdrawal_notice ?? [],
    signature: raw.signature
      ? {
          method: raw.signature.method,
          signerName: raw.signature.signer_name,
          signedAt: raw.signature.signed_at,
          earlyStartRequested: raw.signature.early_start_requested,
          withdrawalEndsOn: raw.signature.withdrawal_ends_on,
        }
      : null,
  };
}

interface SigningInput {
  readonly signature: SignatureValue;
  readonly noticeAcknowledged: boolean;
  readonly earlyStart: boolean;
}

function signBody(input: SigningInput) {
  return {
    method: input.signature.method,
    image_png: input.signature.imagePngBase64,
    ...(input.signature.typedName === undefined
      ? {}
      : { typed_name: input.signature.typedName }),
    withdrawal_notice_acknowledged: input.noticeAcknowledged,
    early_start_requested: input.earlyStart,
  };
}

async function request(url: string, init: RequestInit): Promise<SigningView> {
  const response = await transportFetch(url, {
    ...init,
    cache: "no-store",
    headers: { "Content-Type": "application/json" },
  });
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `Care contract signing failed: ${response.status}`,
    );
  }
  const body = (await response.json()) as { data: BackendSigningView };
  return mapSigningView(body.data);
}

/** Where a contract to sign comes from: the mail link or the inbox. */
export type SigningSource =
  | { readonly kind: "link"; readonly link: string }
  | {
      readonly kind: "inbox";
      readonly organizationId: string;
      readonly contractId: string;
    };

function basePath(source: SigningSource): string {
  return source.kind === "link"
    ? `/api/public/care-contract-signing/${encodeURIComponent(source.link)}`
    : `/api/parent/me/documents/${encodeURIComponent(source.organizationId)}/contracts/${encodeURIComponent(source.contractId)}`;
}

/** Reads the contract. Reading never uses the mail link up. */
export function fetchSigningView(source: SigningSource): Promise<SigningView> {
  const url =
    source.kind === "link" ? basePath(source) : `${basePath(source)}/signing`;
  return request(url, { method: "GET" });
}

/** Signs the contract; the mail link is closed afterwards. */
export function signContract(
  source: SigningSource,
  input: SigningInput,
): Promise<SigningView> {
  return request(`${basePath(source)}/sign`, {
    method: "POST",
    body: JSON.stringify(signBody(input)),
  });
}

/** The address of the frozen PDF the parents sign. */
export function signingDocumentUrl(source: SigningSource): string {
  return source.kind === "link"
    ? `${basePath(source)}/document`
    : `${basePath(source)}/pdf`;
}

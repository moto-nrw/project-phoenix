/**
 * Client of the care contracts in the Träger portal (#4678, ADR 0153): the
 * contract template of every offering, the drafts from the accepted
 * applications, the PDF, the send by mail and into the parents app, and the
 * discard of a draft. Backend int64 IDs leave here as strings; money stays
 * whole cent; calendar dates stay "YYYY-MM-DD".
 */

import type { MailOutcome } from "~/lib/email-delivery-status";
import { apiErrorFromResponse, transportFetch } from "./api-error";
import { filenameFromDisposition } from "./file-download";
import { traegerRequest } from "./traeger-api";

const CONTRACTS_PATH = "/api/traeger/contracts";
const TEMPLATES_PATH = "/api/traeger/contract-templates";

/**
 * Every state of ADR 0088. The office reaches draft and sent; the conclusion
 * at the acceptance (#4680) or the signature (#4679) reaches active; a
 * termination or a withdrawal (#4682) ends an active contract.
 */
export type CareContractStatus =
  "entwurf" | "versendet" | "aktiv" | "gekuendigt" | "widerrufen" | "beendet";

export type ContractTermKind = "open_ended" | "school_year" | "fixed_months";
export type ContractNoticeTo = "month_end" | "half_year" | "school_year_end";
export type Weekday = "mon" | "tue" | "wed" | "thu" | "fri" | "sat" | "sun";

export interface ContractTerms {
  readonly termKind: ContractTermKind;
  readonly termMonths: number | null;
  readonly noticeMonths: number;
  readonly noticeTo: ContractNoticeTo;
  readonly specialTermination: boolean;
  readonly specialTerminationNote: string;
  readonly statuteRequired: boolean;
  readonly statuteTitle: string;
  readonly statuteUrl: string;
  /** Empty: the Träger itself. */
  readonly contractPartner: string;
}

interface ContractTemplate extends ContractTerms {
  readonly id: string;
  readonly updatedAt: string;
}

/** One offering of a school with its template, or none yet. */
export interface OfferingTemplate {
  readonly acceptanceText?: string;
  /**
   * The withdrawal notice and the block for an early start the school adds
   * to the enrollment form (#4682); empty like `acceptanceText`.
   */
  readonly withdrawalText?: string;
  readonly earlyStartText?: string;
  /** The term and the notice period as the PDF prints them. */
  readonly termText: string;
  readonly noticeText: string;
  readonly schoolId: string;
  readonly schoolName: string;
  readonly offeringId: string;
  readonly offeringName: string;
  readonly phaseName: string;
  readonly priceCents: number | null;
  readonly active: boolean;
  readonly template: ContractTemplate | null;
}

/** One contract per accepted application child at its school. */
export interface CandidateKey {
  readonly schoolId: string;
  readonly requestChildId: string;
}

/**
 * One booked service: on a weekday the module (a care offering) and its end
 * time "HH:MM", empty when the module has none. A weekday without a service
 * has no care; a weekday may carry several modules.
 */
export interface ContractService {
  readonly weekday: Weekday;
  readonly careOfferingId: string;
  readonly moduleName: string;
  readonly endsAt: string;
  /**
   * Last booked day "YYYY-MM-DD" of a module that ends before the contract
   * version; null when it runs as long as the version.
   */
  readonly lastDay: string | null;
}

/** Why no draft can be made for a candidate yet. */
export type TemplateIssue =
  | "template_missing"
  | "templates_differ"
  | "conclusion_automatic"
  | "acceptance_missing"
  | "acceptance_missing_renewal"
  | "acceptance_changed"
  | "recipient_missing"
  | "mail_blocked"
  | "mail_stopped";

/**
 * Why the application gives no proof of the parents' agreement for a
 * conclusion at the acceptance (#4680); null when it does.
 */
type AcceptanceIssue = "acceptance_missing" | "acceptance_changed";

/** An accepted application child with booked care and no contract. */
export interface ContractCandidate extends CandidateKey {
  readonly acceptanceIssue: AcceptanceIssue | null;
  /**
   * A rollover renewed the child into a new phase; a missing acceptance then
   * means the parents have not accepted that phase's conditions (#6456).
   */
  readonly renewal: boolean;
  readonly schoolName: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly guardianName: string;
  readonly guardianEmail: string;
  readonly phaseName: string;
  readonly services: readonly ContractService[];
  readonly effectiveFrom: string;
  readonly templateIssue: TemplateIssue | null;
  /** The booked modules as the contract names them: „A, B und C“. */
  readonly modulesText: string;
  /**
   * Why a mail to the parents' address would stay back now, for a child the
   * conclusion at the acceptance could conclude (#4680); null otherwise.
   */
  readonly mailBlock: "stopped" | "address" | null;
}

/**
 * How a contract was concluded (ADR 0088 section 3): at the acceptance of
 * the application, or with the parents' signature.
 */
export type ConclusionMethod =
  "mit_anmeldung" | "einfach_elektronisch" | "papier";

export interface ContractConclusion {
  readonly method: ConclusionMethod;
  readonly at: string;
}

/**
 * How the contracts of a school come about (#4680): sent later by the
 * office (Weg a) or concluded at the acceptance (Weg b).
 */
export type ConclusionMode = "nachtraeglich" | "bei_annahme";

export interface SchoolConclusionMode {
  readonly schoolId: string;
  readonly schoolName: string;
  readonly mode: ConclusionMode;
}

export interface CareContractSummary {
  readonly id: string;
  readonly status: CareContractStatus;
  readonly schoolId: string;
  readonly schoolName: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly moduleNames: readonly string[];
  /** The modules as the contract names them: „A, B und C“. */
  readonly modulesText: string;
  readonly partyName: string;
  readonly partyEmail: string;
  readonly effectiveFrom: string;
  readonly createdAt: string;
  readonly sentAt: string | null;
  /** Null until the contract is concluded. */
  readonly concluded: ContractConclusion | null;
  /** Last day of a terminated contract (#4682); null otherwise. */
  readonly endsOn: string | null;
}

export interface CareContractList {
  readonly contracts: readonly CareContractSummary[];
  readonly counts: Readonly<Partial<Record<CareContractStatus, number>>>;
  readonly hasMore: boolean;
}

export type ContractMailState = "queued" | "sent" | "failed" | "unknown";

/** The state of a contract version (ADR 0088 section 2, #4681). */
type ContractVersionStatus =
  "beantragt" | "wirksam" | "abgeloest" | "abgelehnt" | "ersetzt";

/** Where a version came from: the conclusion or an approved change. */
type ContractVersionSource = "abschluss" | "buchungsaenderung";

export interface ContractVersion {
  readonly number: number;
  readonly status: ContractVersionStatus;
  readonly source: ContractVersionSource;
  readonly effectiveFrom: string;
  readonly endsOn: string | null;
  /** The last day it holds; null while it holds open ended. */
  readonly validUntil: string | null;
  readonly services: readonly ContractService[];
  readonly createdAt: string;
}

export interface CareContractDetail extends CareContractSummary {
  readonly version: {
    readonly number: number;
    readonly effectiveFrom: string;
    readonly endsOn: string | null;
    readonly childBirthDate: string | null;
    readonly schoolName: string;
    readonly phaseName: string;
    readonly services: readonly ContractService[];
    readonly priceCents: number | null;
    readonly terms: ContractTerms;
    readonly createdAt: string;
  };
  readonly parties: readonly {
    readonly firstName: string;
    readonly lastName: string;
    readonly email: string;
    readonly hasAccount: boolean;
  }[];
  readonly mailings: readonly {
    readonly recipient: string;
    readonly sentAt: string;
    readonly state: ContractMailState;
    readonly outcome?: MailOutcome;
    /** Why another mail to this address would stay back now. */
    readonly sendBlock?: ContractSendBlock;
  }[];
  readonly documentFrozen: boolean;
  /** Every version, oldest first. */
  readonly versions: readonly ContractVersion[];
  /** The signature of the current version (#4679); null until signed. */
  readonly signature: ContractSignature | null;
  /** The reminders of a contract that waits for its signature. */
  readonly reminders: readonly {
    readonly number: number;
    readonly sentAt: string;
  }[];
  /**
   * Term up to the contract's last day, notice period and monthly fee of the
   * version above, as the contract PDF prints them.
   */
  readonly termText: string;
  readonly noticeText: string;
  readonly feeText: string;
}

export interface ContractSignature {
  readonly method: "drawn" | "typed" | "paper";
  readonly channel: "parent_account" | "mail_link" | "office";
  readonly signerName: string;
  readonly signedAt: string;
  readonly signedOn: string;
  readonly documentSha256: string;
  readonly earlyStartRequested: boolean;
  readonly withdrawalEndsOn: string | null;
  /** The drawn or typed signature as a base64 PNG; empty on paper. */
  readonly imagePng: string;
  readonly paperFilename: string;
  readonly intact: boolean;
}

type CreateSkipReason =
  TemplateIssue | "contract_exists" | "application_not_found";

export interface CreateResult {
  readonly created: readonly CareContractSummary[];
  readonly skipped: readonly (CandidateKey & {
    readonly reason: CreateSkipReason;
  })[];
}

export type SendSkipReason =
  | "recipient_missing"
  | "partner_missing"
  | "mail_pending"
  | "mail_delayed"
  | "mail_blocked"
  | "mail_block_unreadable"
  | "not_sendable";

type ContractSendBlock =
  "delayed" | "stopped" | "address" | "provider" | "provider_unknown";

export interface SendResult {
  readonly sent: readonly string[];
  readonly skipped: readonly {
    readonly id: string;
    readonly reason: SendSkipReason;
  }[];
}

export interface ContractFile {
  readonly blob: Blob;
  readonly filename: string;
}

interface BackendTerms {
  term_kind: ContractTermKind;
  term_months: number | null;
  notice_months: number;
  notice_to: ContractNoticeTo;
  special_termination: boolean;
  special_termination_note: string;
  statute_required: boolean;
  statute_title: string;
  statute_url: string;
  contract_partner: string;
}

interface BackendTemplate extends BackendTerms {
  id: number;
  updated_at: string;
}

interface BackendOfferingTemplate {
  acceptance_text?: string;
  withdrawal_text?: string;
  early_start_text?: string;
  term_text?: string;
  notice_text?: string;
  school_id: number;
  school_name: string;
  offering_id: number;
  offering_name: string;
  phase_name: string;
  price_cents: number | null;
  active: boolean;
  template: BackendTemplate | null;
}

interface BackendService {
  weekday: Weekday;
  care_offering_id: number;
  module_name: string;
  ends_at: string;
  last_day?: string | null;
}

interface BackendCandidate {
  acceptance_issue?: AcceptanceIssue | "";
  renewal?: boolean;
  mail_block?: "stopped" | "address" | "";
  school_id: number;
  request_child_id: number;
  school_name: string;
  child_first_name: string;
  child_last_name: string;
  guardian_name: string;
  guardian_email: string;
  phase_name: string;
  services: BackendService[] | null;
  effective_from: string;
  template_issue: TemplateIssue | "";
  modules_text?: string;
}

interface BackendSummary {
  id: number;
  status: CareContractStatus;
  school_id: number;
  school_name: string;
  child_first_name: string;
  child_last_name: string;
  module_names: string[] | null;
  modules_text?: string;
  party_name: string;
  party_email: string;
  effective_from: string;
  created_at: string;
  sent_at: string | null;
  concluded?: { method: ConclusionMethod; at: string } | null;
  ends_on?: string | null;
}

interface BackendDetail extends BackendSummary {
  version: {
    number: number;
    effective_from: string;
    ends_on: string | null;
    child_birth_date: string | null;
    school_name: string;
    phase_name: string;
    services: BackendService[] | null;
    price_cents: number | null;
    terms: BackendTerms;
    created_at: string;
  };
  parties:
    | {
        first_name: string;
        last_name: string;
        email: string;
        has_account: boolean;
      }[]
    | null;
  mailings:
    | {
        recipient: string;
        sent_at: string;
        state: ContractMailState;
        outcome?: MailOutcome;
        send_block?: ContractSendBlock;
      }[]
    | null;
  document_frozen: boolean;
  signature?: {
    method: ContractSignature["method"];
    channel: ContractSignature["channel"];
    signer_name: string;
    signed_at: string;
    signed_on: string;
    document_sha256: string;
    early_start_requested: boolean;
    withdrawal_ends_on: string | null;
    image_png: string | null;
    paper_filename: string;
    intact: boolean;
  } | null;
  reminders?: { number: number; sent_at: string }[] | null;
  versions?: BackendVersion[] | null;
  term_text?: string;
  notice_text?: string;
  fee_text?: string;
}

interface BackendVersion {
  number: number;
  status: ContractVersionStatus;
  source: ContractVersionSource;
  effective_from: string;
  ends_on: string | null;
  valid_until: string | null;
  services: BackendService[] | null;
  created_at: string;
}

function termsFromBackend(terms: BackendTerms): ContractTerms {
  return {
    termKind: terms.term_kind,
    termMonths: terms.term_months,
    noticeMonths: terms.notice_months,
    noticeTo: terms.notice_to,
    specialTermination: terms.special_termination,
    specialTerminationNote: terms.special_termination_note,
    statuteRequired: terms.statute_required,
    statuteTitle: terms.statute_title,
    statuteUrl: terms.statute_url,
    contractPartner: terms.contract_partner,
  };
}

function termsToBackend(terms: ContractTerms): BackendTerms {
  return {
    term_kind: terms.termKind,
    term_months: terms.termKind === "fixed_months" ? terms.termMonths : null,
    notice_months: terms.noticeMonths,
    notice_to: terms.noticeTo,
    special_termination: terms.specialTermination,
    special_termination_note: terms.specialTerminationNote,
    statute_required: terms.statuteRequired,
    statute_title: terms.statuteTitle,
    statute_url: terms.statuteUrl,
    contract_partner: terms.contractPartner,
  };
}

function offeringFromBackend(row: BackendOfferingTemplate): OfferingTemplate {
  return {
    acceptanceText: row.acceptance_text ?? "",
    withdrawalText: row.withdrawal_text ?? "",
    earlyStartText: row.early_start_text ?? "",
    termText: row.term_text ?? "",
    noticeText: row.notice_text ?? "",
    schoolId: String(row.school_id),
    schoolName: row.school_name,
    offeringId: String(row.offering_id),
    offeringName: row.offering_name,
    phaseName: row.phase_name,
    priceCents: row.price_cents,
    active: row.active,
    template: row.template
      ? {
          ...termsFromBackend(row.template),
          id: String(row.template.id),
          updatedAt: row.template.updated_at,
        }
      : null,
  };
}

function servicesFromBackend(rows: BackendService[] | null): ContractService[] {
  return (rows ?? []).map((row) => ({
    weekday: row.weekday,
    careOfferingId: String(row.care_offering_id),
    moduleName: row.module_name,
    endsAt: row.ends_at,
    lastDay: row.last_day ?? null,
  }));
}

function candidateFromBackend(row: BackendCandidate): ContractCandidate {
  return {
    schoolId: String(row.school_id),
    requestChildId: String(row.request_child_id),
    schoolName: row.school_name,
    childFirstName: row.child_first_name,
    childLastName: row.child_last_name,
    guardianName: row.guardian_name,
    guardianEmail: row.guardian_email,
    phaseName: row.phase_name,
    services: servicesFromBackend(row.services),
    effectiveFrom: row.effective_from,
    templateIssue: row.template_issue === "" ? null : row.template_issue,
    modulesText: row.modules_text ?? "",
    acceptanceIssue: row.acceptance_issue ? row.acceptance_issue : null,
    renewal: row.renewal === true,
    mailBlock: row.mail_block ? row.mail_block : null,
  };
}

function summaryFromBackend(row: BackendSummary): CareContractSummary {
  return {
    id: String(row.id),
    status: row.status,
    schoolId: String(row.school_id),
    schoolName: row.school_name,
    childFirstName: row.child_first_name,
    childLastName: row.child_last_name,
    moduleNames: row.module_names ?? [],
    modulesText: row.modules_text ?? "",
    partyName: row.party_name,
    partyEmail: row.party_email,
    effectiveFrom: row.effective_from,
    createdAt: row.created_at,
    sentAt: row.sent_at,
    concluded: row.concluded
      ? { method: row.concluded.method, at: row.concluded.at }
      : null,
    endsOn: row.ends_on ?? null,
  };
}

function detailFromBackend(row: BackendDetail): CareContractDetail {
  return {
    ...summaryFromBackend(row),
    version: {
      number: row.version.number,
      effectiveFrom: row.version.effective_from,
      endsOn: row.version.ends_on,
      childBirthDate: row.version.child_birth_date,
      schoolName: row.version.school_name,
      phaseName: row.version.phase_name,
      services: servicesFromBackend(row.version.services),
      priceCents: row.version.price_cents,
      terms: termsFromBackend(row.version.terms),
      createdAt: row.version.created_at,
    },
    parties: (row.parties ?? []).map((party) => ({
      firstName: party.first_name,
      lastName: party.last_name,
      email: party.email,
      hasAccount: party.has_account,
    })),
    mailings: (row.mailings ?? []).map((mailing) => ({
      recipient: mailing.recipient,
      sentAt: mailing.sent_at,
      outcome: mailing.outcome,
      sendBlock: mailing.send_block,
      state: mailing.state,
    })),
    documentFrozen: row.document_frozen,
    signature: row.signature
      ? {
          method: row.signature.method,
          channel: row.signature.channel,
          signerName: row.signature.signer_name,
          signedAt: row.signature.signed_at,
          signedOn: row.signature.signed_on,
          documentSha256: row.signature.document_sha256,
          earlyStartRequested: row.signature.early_start_requested,
          withdrawalEndsOn: row.signature.withdrawal_ends_on,
          imagePng: row.signature.image_png ?? "",
          paperFilename: row.signature.paper_filename,
          intact: row.signature.intact,
        }
      : null,
    reminders: (row.reminders ?? []).map((reminder) => ({
      number: reminder.number,
      sentAt: reminder.sent_at,
    })),
    versions: (row.versions ?? []).map((version) => ({
      number: version.number,
      status: version.status,
      source: version.source,
      effectiveFrom: version.effective_from,
      endsOn: version.ends_on,
      validUntil: version.valid_until,
      services: servicesFromBackend(version.services),
      createdAt: version.created_at,
    })),
    termText: row.term_text ?? "",
    noticeText: row.notice_text ?? "",
    feeText: row.fee_text ?? "",
  };
}

function keyToBackend(key: CandidateKey) {
  return {
    school_id: Number(key.schoolId),
    request_child_id: Number(key.requestChildId),
  };
}

function contractPath(id: string): string {
  return `${CONTRACTS_PATH}/${encodeURIComponent(id)}`;
}

export async function fetchCareContracts(
  filter: {
    status?: CareContractStatus;
    schoolId?: string;
    search?: string;
    offset?: number;
  } = {},
): Promise<CareContractList> {
  const query = new URLSearchParams();
  if (filter.status) query.set("status", filter.status);
  if (filter.schoolId) query.set("school_id", filter.schoolId);
  if (filter.search) query.set("search", filter.search);
  if (filter.offset) query.set("offset", String(filter.offset));
  const path = query.size ? `${CONTRACTS_PATH}?${query}` : CONTRACTS_PATH;
  const body = await traegerRequest<{
    contracts: BackendSummary[] | null;
    counts: Partial<Record<CareContractStatus, number>> | null;
    has_more: boolean;
  }>(path);
  return {
    contracts: (body?.contracts ?? []).map(summaryFromBackend),
    counts: body?.counts ?? {},
    hasMore: body?.has_more ?? false,
  };
}

export async function fetchConclusionModes(): Promise<SchoolConclusionMode[]> {
  const rows = await traegerRequest<
    { school_id: number; school_name: string; mode: ConclusionMode }[] | null
  >(`${CONTRACTS_PATH}/conclusion-modes`);
  return (rows ?? []).map((row) => ({
    schoolId: String(row.school_id),
    schoolName: row.school_name,
    mode: row.mode,
  }));
}

export async function fetchContractCandidates(): Promise<ContractCandidate[]> {
  const rows = await traegerRequest<BackendCandidate[] | null>(
    `${CONTRACTS_PATH}/candidates`,
  );
  return (rows ?? []).map(candidateFromBackend);
}

// The backend accepts at most 200 items per request. Sequential batches
// keep large selections within that contract without concurrent PDF rendering.
const MAX_BATCH = 200;

export async function createCareContracts(
  keys: readonly CandidateKey[],
): Promise<CreateResult> {
  const result: {
    created: CareContractSummary[];
    skipped: CreateResult["skipped"][number][];
  } = { created: [], skipped: [] };
  for (let offset = 0; offset < keys.length; offset += MAX_BATCH) {
    const batch = await createCareContractBatch(
      keys.slice(offset, offset + MAX_BATCH),
    );
    result.created.push(...batch.created);
    result.skipped.push(...batch.skipped);
  }
  return result;
}

export async function sendCareContracts(
  ids: readonly string[],
): Promise<SendResult> {
  const result: { sent: string[]; skipped: SendResult["skipped"][number][] } = {
    sent: [],
    skipped: [],
  };
  for (let offset = 0; offset < ids.length; offset += MAX_BATCH) {
    const batch = await sendCareContractBatch(
      ids.slice(offset, offset + MAX_BATCH),
    );
    result.sent.push(...batch.sent);
    result.skipped.push(...batch.skipped);
  }
  return result;
}

async function createCareContractBatch(
  keys: readonly CandidateKey[],
): Promise<CreateResult> {
  const body = await traegerRequest<{
    created: BackendSummary[] | null;
    skipped: (BackendCandidate & { reason: CreateSkipReason })[] | null;
  }>(CONTRACTS_PATH, {
    method: "POST",
    body: { contracts: keys.map(keyToBackend) },
  });
  return {
    created: (body?.created ?? []).map(summaryFromBackend),
    skipped: (body?.skipped ?? []).map((skip) => ({
      schoolId: String(skip.school_id),
      requestChildId: String(skip.request_child_id),
      reason: skip.reason,
    })),
  };
}

/** Retries a failed delivery at a corrected address, keeping its PDF. */
export function resendCareContract(
  id: string,
  email: string,
): Promise<SendResult> {
  return sendCareContractBatch([id], email);
}

async function sendCareContractBatch(
  ids: readonly string[],
  recipientEmail?: string,
): Promise<SendResult> {
  const body = await traegerRequest<{
    sent: number[] | null;
    skipped: { id: number; reason: SendSkipReason }[] | null;
  }>(`${CONTRACTS_PATH}/send`, {
    method: "POST",
    body: {
      contract_ids: ids.map(Number),
      ...(recipientEmail === undefined
        ? {}
        : { recipient_email: recipientEmail }),
    },
  });
  return {
    sent: (body?.sent ?? []).map(String),
    skipped: (body?.skipped ?? []).map((skip) => ({
      id: String(skip.id),
      reason: skip.reason,
    })),
  };
}

export async function fetchCareContract(
  id: string,
): Promise<CareContractDetail> {
  return detailFromBackend(
    await traegerRequest<BackendDetail>(contractPath(id)),
  );
}

export async function discardCareContract(id: string): Promise<void> {
  await traegerRequest<unknown>(contractPath(id), { method: "DELETE" });
}

/** The PDF: a preview of a draft, the stored document once sent. */
export async function downloadCareContract(id: string): Promise<ContractFile> {
  return downloadContractFile(`${contractPath(id)}/document`);
}

/**
 * The PDF of one version (#4681): the stored one when it was sent, else a
 * rendering with the current letterhead.
 */
export async function downloadCareContractVersion(
  id: string,
  number: number,
): Promise<ContractFile> {
  return downloadContractFile(
    `${contractPath(id)}/versions/${encodeURIComponent(String(number))}/document`,
  );
}

async function downloadContractFile(
  path: string,
  fallbackFilename = "betreuungsvertrag.pdf",
): Promise<ContractFile> {
  const response = await transportFetch(path, {
    credentials: "include",
    cache: "no-store",
  });
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `Request failed (${response.status})`,
    );
  }
  return {
    blob: await response.blob(),
    filename: filenameFromDisposition(response) ?? fallbackFilename,
  };
}

export async function fetchContractTemplates(): Promise<OfferingTemplate[]> {
  const rows = await traegerRequest<BackendOfferingTemplate[] | null>(
    TEMPLATES_PATH,
  );
  return (rows ?? []).map(offeringFromBackend);
}

export async function saveContractTemplate(
  schoolId: string,
  offeringId: string,
  terms: ContractTerms,
): Promise<OfferingTemplate> {
  return offeringFromBackend(
    await traegerRequest<BackendOfferingTemplate>(
      `${TEMPLATES_PATH}/${encodeURIComponent(schoolId)}/${encodeURIComponent(offeringId)}`,
      { method: "PUT", body: termsToBackend(terms) },
    ),
  );
}

/**
 * Records a contract the parents signed on paper (#4679): the signed PDF,
 * who signed and on which day. The contract is concluded afterwards.
 */
export async function recordPaperSignature(
  id: string,
  input: {
    readonly file: File;
    readonly signerName: string;
    readonly signedOn: string;
  },
): Promise<void> {
  const form = new FormData();
  form.append("file", input.file);
  form.append("signer_name", input.signerName);
  form.append("signed_on", input.signedOn);
  const response = await transportFetch(`${contractPath(id)}/paper-signature`, {
    method: "POST",
    credentials: "include",
    cache: "no-store",
    body: form,
  });
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `Request failed (${response.status})`,
    );
  }
}

export async function downloadPaperSignature(
  id: string,
): Promise<ContractFile> {
  return downloadContractFile(
    `${contractPath(id)}/paper-signature`,
    "vertrag-papier.pdf",
  );
}

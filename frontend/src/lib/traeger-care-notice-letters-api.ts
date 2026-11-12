/**
 * Client of the stored originals of the Jahresbescheide (#6347): the office
 * stores the PDF of every issued notice once, then sends it by mail and to
 * the parents inbox, or prints it. Backend IDs leave here as strings.
 */

import type {
  MailOutcome,
  MailProviderBlock,
} from "~/lib/email-delivery-status";
import {
  type BillingFile,
  type LetterMailStatus,
  billingFile,
} from "./traeger-billing-api";
import type { CareNoticeKind } from "./traeger-care-notices-api";
import { traegerRequest } from "./traeger-api";

const RUNS_PATH = "/api/traeger/care-notice-runs";

export interface CareNoticeMailing {
  readonly noticeId: string;
  readonly contractId: string;
  readonly kind: CareNoticeKind;
  readonly number: string;
  /** The number of the notice a change notice amends, empty otherwise. */
  readonly amendsNumber: string;
  /** The number of the change notice that amends this one, empty otherwise. */
  readonly amendedByNumber: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly schoolName: string;
  readonly recipientFirstName: string;
  readonly recipientLastName: string;
  readonly recipientEmail: string;
  readonly totalCents: number;
  readonly hasLetter: boolean;
  readonly sha256: string;
  readonly inInbox: boolean;
  readonly status: LetterMailStatus;
  readonly sends: number;
  readonly createdAt: string | null;
  readonly requestedAt: string | null;
  readonly sentAt: string | null;
  readonly mailOutcome?: MailOutcome;
  readonly providerBlock?: MailProviderBlock;
}

export interface CareNoticeLetterSummary {
  readonly notices: number;
  readonly letters: number;
  readonly sent: number;
  readonly queued: number;
  readonly failed: number;
  readonly notSent: number;
  readonly withoutEmail: number;
  readonly withoutInbox: number;
}

export interface CareNoticeLetters {
  readonly notices: readonly CareNoticeMailing[];
  readonly summary: CareNoticeLetterSummary;
}

export interface CareNoticeSeries {
  readonly queued: number;
  readonly already: number;
  readonly withoutEmail: number;
  readonly recipientChanged: number;
}

interface BackendMailing {
  notice_id: number;
  contract_id: number;
  kind: CareNoticeKind;
  number: string;
  amends_number?: string;
  amended_by_number?: string;
  child_first_name: string;
  child_last_name: string;
  school_name: string;
  recipient_first_name: string;
  recipient_last_name: string;
  recipient_email: string;
  total_cents: number;
  has_letter: boolean;
  sha256?: string;
  in_inbox: boolean;
  status: LetterMailStatus;
  sends: number;
  created_at: string | null;
  requested_at: string | null;
  sent_at: string | null;
  mail_outcome?: MailOutcome;
  provider_block?: MailProviderBlock;
}

interface BackendLetters {
  notices: BackendMailing[];
  summary: {
    notices: number;
    letters: number;
    sent: number;
    queued: number;
    failed: number;
    not_sent: number;
    without_email: number;
    without_inbox: number;
  };
}

function mailingFromBackend(mailing: BackendMailing): CareNoticeMailing {
  return {
    noticeId: String(mailing.notice_id),
    contractId: String(mailing.contract_id),
    kind: mailing.kind,
    number: mailing.number,
    amendsNumber: mailing.amends_number ?? "",
    amendedByNumber: mailing.amended_by_number ?? "",
    childFirstName: mailing.child_first_name,
    childLastName: mailing.child_last_name,
    schoolName: mailing.school_name,
    recipientFirstName: mailing.recipient_first_name,
    recipientLastName: mailing.recipient_last_name,
    recipientEmail: mailing.recipient_email,
    totalCents: mailing.total_cents,
    hasLetter: mailing.has_letter,
    sha256: mailing.sha256 ?? "",
    inInbox: mailing.in_inbox,
    status: mailing.status,
    sends: mailing.sends,
    createdAt: mailing.created_at,
    requestedAt: mailing.requested_at,
    sentAt: mailing.sent_at,
    mailOutcome: mailing.mail_outcome,
    providerBlock: mailing.provider_block,
  };
}

function lettersFromBackend(letters: BackendLetters): CareNoticeLetters {
  const { summary } = letters;
  return {
    notices: (letters.notices ?? []).map(mailingFromBackend),
    summary: {
      notices: summary.notices,
      letters: summary.letters,
      sent: summary.sent,
      queued: summary.queued,
      failed: summary.failed,
      notSent: summary.not_sent,
      withoutEmail: summary.without_email,
      withoutInbox: summary.without_inbox,
    },
  };
}

function lettersPath(runId: string): string {
  return `${RUNS_PATH}/${encodeURIComponent(runId)}/letters`;
}

function noticePath(runId: string, contractId: string): string {
  return `${RUNS_PATH}/${encodeURIComponent(runId)}/notices/${encodeURIComponent(contractId)}`;
}

/** The originals of a released run with the state of every mail. */
export async function fetchCareNoticeLetters(
  runId: string,
): Promise<CareNoticeLetters> {
  return lettersFromBackend(
    await traegerRequest<BackendLetters>(lettersPath(runId)),
  );
}

/** Stores the original of every notice without one; sends nothing. */
export async function createCareNoticeLetters(
  runId: string,
): Promise<CareNoticeLetters> {
  return lettersFromBackend(
    await traegerRequest<BackendLetters>(lettersPath(runId), {
      method: "POST",
    }),
  );
}

/** Mails every stored notice with an address and no mail yet. */
export async function mailCareNoticeLetters(
  runId: string,
): Promise<CareNoticeSeries> {
  const series = await traegerRequest<{
    queued: number;
    already: number;
    without_email: number;
    recipient_changed?: number;
  }>(`${lettersPath(runId)}/mail`, { method: "POST" });
  return {
    queued: series.queued,
    already: series.already,
    withoutEmail: series.without_email,
    recipientChanged: series.recipient_changed ?? 0,
  };
}

/** Mails the notice of a contract, first or again. */
export async function mailCareNoticeLetter(
  runId: string,
  contractId: string,
): Promise<CareNoticeMailing> {
  return mailingFromBackend(
    await traegerRequest<BackendMailing>(
      `${noticePath(runId, contractId)}/mail`,
      { method: "POST" },
    ),
  );
}

/** The stored original of the notice of a contract. */
export async function downloadCareNoticePDF(
  runId: string,
  contractId: string,
): Promise<BillingFile> {
  return billingFile(
    `${noticePath(runId, contractId)}/pdf`,
    {},
    "gebuehrenbescheid.pdf",
  );
}

/** The stored originals of the run in one PDF to print. */
export async function printCareNoticeLetters(
  runId: string,
  withoutEmailOnly = false,
): Promise<BillingFile> {
  return billingFile(
    `${lettersPath(runId)}/print${withoutEmailOnly ? "?only=without_email" : ""}`,
    {},
    "jahresbescheide.pdf",
  );
}

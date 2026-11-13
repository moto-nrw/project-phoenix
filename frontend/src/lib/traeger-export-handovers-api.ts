/**
 * Client of the handover of finance exports to the recipients of the Träger
 * (#6824, ADR 0218; #6911, ADR 0221): the recipients without their
 * passwords, the recipient of each format, their newest handovers, and the
 * handover of a Sollstellung, a DATEV booking batch or a PAIN.008 file. Backend int64 IDs leave here as strings, instants as ISO
 * strings.
 */

import type {
  ExportFormat,
  ExportRunPathKind,
  HandoverState,
} from "./traeger-finance-exports-api";
import { traegerRequest } from "./traeger-api";

const RECIPIENTS = "/api/traeger/export-recipient";
const HANDOVERS = "/api/traeger/finance-handovers";

/** The fields of a recipient, in form order; the backend names these. */
export type RecipientField =
  | "name"
  | "host"
  | "port"
  | "username"
  | "password"
  | "remote_directory"
  | "host_key_fingerprint";

export type { HandoverState } from "./traeger-finance-exports-api";

export type HandoverFormat = Extract<
  ExportFormat,
  "sollstellung" | "datev" | "pain008"
>;

/** The formats a recipient can receive, in the order the Träger sees them. */
export const HANDOVER_FORMATS: readonly HandoverFormat[] = [
  "sollstellung",
  "datev",
  "pain008",
];

export interface ExportHandover {
  readonly id: string;
  readonly createdAt: string;
  readonly format: ExportFormat;
  readonly fileName: string;
  readonly byteSize: number;
  readonly targetHost: string;
  readonly targetDirectory: string;
  /** Null once the recipient was removed; the name stays as it was then. */
  readonly recipientId: string | null;
  readonly recipientName: string;
  readonly state: HandoverState;
  /** Stable reason code of a failure, empty otherwise. */
  readonly reason: string;
}

export interface ExportRecipient {
  readonly id: string;
  readonly name: string;
  readonly enabled: boolean;
  /** Switched on and complete: a handover can start. */
  readonly ready: boolean;
  readonly host: string;
  readonly port: number;
  readonly username: string;
  readonly remoteDirectory: string;
  readonly hostKeyFingerprint: string;
  /** A password is stored; it never comes back. */
  readonly passwordSet: boolean;
  readonly missingFields: readonly RecipientField[];
  /** The formats this recipient receives, in order. */
  readonly formats: readonly HandoverFormat[];
  /** Its newest handover: its error state until the next one. */
  readonly lastHandover: ExportHandover | null;
  readonly updatedAt: string;
}

/** The recipient of one format; null leaves it to the download. */
export interface ExportFormatAssignment {
  readonly format: HandoverFormat;
  readonly recipientId: string | null;
}

/** Every recipient, the recipient of each format and the newest handovers. */
export interface ExportRecipientsPage {
  readonly recipients: readonly ExportRecipient[];
  readonly assignments: readonly ExportFormatAssignment[];
  readonly handovers: readonly ExportHandover[];
}

/** One recipient with its own newest handovers. */
export interface ExportRecipientPage {
  readonly recipient: ExportRecipient;
  readonly handovers: readonly ExportHandover[];
}

/** What the Träger saves. Without a password the stored one stays. */
export interface ExportRecipientDraft {
  readonly name: string;
  readonly enabled: boolean;
  readonly host: string;
  readonly port: number;
  readonly username: string;
  readonly password?: string;
  readonly remoteDirectory: string;
  readonly hostKeyFingerprint: string;
}

export interface HandoverOptions {
  /** Sends a content again that already reached the recipient. */
  readonly repeat?: boolean;
  /** DATEV: every booking again, in the fiscal year starting then. */
  readonly all?: boolean;
  readonly fiscalYear?: string;
}

export interface HandoverResult {
  readonly fileName: string;
  /** transferred, failed or unknown; a failure is a result, not an error. */
  readonly state: HandoverState;
  readonly reason: string;
  readonly recipientName: string;
  readonly targetHost: string;
  readonly targetDirectory: string;
}

interface BackendHandover {
  id: number;
  created_at: string;
  format: ExportFormat;
  filename: string;
  byte_size: number;
  target_host: string;
  target_directory: string;
  recipient_id?: number | null;
  recipient_name?: string;
  state: HandoverState;
  reason?: string;
}

interface BackendRecipient {
  id: number;
  name: string;
  enabled: boolean;
  ready: boolean;
  host: string;
  port: number;
  username: string;
  remote_directory: string;
  host_key_fingerprint: string;
  password_set: boolean;
  missing_fields: RecipientField[] | null;
  formats: HandoverFormat[] | null;
  last_handover?: BackendHandover | null;
  updated_at: string;
}

interface BackendRecipientsPage {
  recipients: BackendRecipient[] | null;
  assignments: { format: HandoverFormat; recipient_id: number | null }[] | null;
  handovers: BackendHandover[] | null;
}

interface BackendResult {
  file_name: string;
  state: HandoverState;
  reason?: string;
  recipient_name?: string;
  target_host: string;
  target_directory: string;
}

function handoverFromBackend(handover: BackendHandover): ExportHandover {
  return {
    id: String(handover.id),
    createdAt: handover.created_at,
    format: handover.format,
    fileName: handover.filename,
    byteSize: handover.byte_size,
    targetHost: handover.target_host,
    targetDirectory: handover.target_directory,
    recipientId:
      handover.recipient_id == null ? null : String(handover.recipient_id),
    recipientName: handover.recipient_name ?? "",
    state: handover.state,
    reason: handover.reason ?? "",
  };
}

function recipientFromBackend(recipient: BackendRecipient): ExportRecipient {
  return {
    id: String(recipient.id),
    name: recipient.name,
    enabled: recipient.enabled,
    ready: recipient.ready,
    host: recipient.host,
    port: recipient.port,
    username: recipient.username,
    remoteDirectory: recipient.remote_directory,
    hostKeyFingerprint: recipient.host_key_fingerprint,
    passwordSet: recipient.password_set,
    missingFields: recipient.missing_fields ?? [],
    formats: recipient.formats ?? [],
    lastHandover: recipient.last_handover
      ? handoverFromBackend(recipient.last_handover)
      : null,
    updatedAt: recipient.updated_at,
  };
}

function recipientsPageFromBackend(
  page: BackendRecipientsPage,
): ExportRecipientsPage {
  return {
    recipients: (page.recipients ?? []).map(recipientFromBackend),
    assignments: (page.assignments ?? []).map((assignment) => ({
      format: assignment.format,
      recipientId:
        assignment.recipient_id == null
          ? null
          : String(assignment.recipient_id),
    })),
    handovers: (page.handovers ?? []).map(handoverFromBackend),
  };
}

function recipientBody(draft: ExportRecipientDraft) {
  return {
    name: draft.name,
    enabled: draft.enabled,
    host: draft.host,
    port: draft.port,
    username: draft.username,
    ...(draft.password ? { password: draft.password } : {}),
    remote_directory: draft.remoteDirectory,
    host_key_fingerprint: draft.hostKeyFingerprint,
  };
}

const recipientPath = (id: string) => `${RECIPIENTS}/${encodeURIComponent(id)}`;

/** Every recipient of the Träger, its formats and the newest handovers. */
export async function fetchExportRecipients(): Promise<ExportRecipientsPage> {
  return recipientsPageFromBackend(
    await traegerRequest<BackendRecipientsPage>(RECIPIENTS),
  );
}

/** One recipient with its own newest handovers. */
export async function fetchExportRecipient(
  id: string,
): Promise<ExportRecipientPage> {
  const page = await traegerRequest<{
    recipient: BackendRecipient;
    handovers: BackendHandover[] | null;
  }>(recipientPath(id));
  return {
    recipient: recipientFromBackend(page.recipient),
    handovers: (page.handovers ?? []).map(handoverFromBackend),
  };
}

/** Adds a recipient; it receives no format until the Träger assigns one. */
export async function createExportRecipient(
  draft: ExportRecipientDraft,
): Promise<ExportRecipient> {
  return recipientFromBackend(
    await traegerRequest<BackendRecipient>(RECIPIENTS, {
      method: "POST",
      body: recipientBody(draft),
    }),
  );
}

/** Saves a recipient; the backend names each wrong field. */
export async function saveExportRecipient(
  id: string,
  draft: ExportRecipientDraft,
): Promise<ExportRecipient> {
  return recipientFromBackend(
    await traegerRequest<BackendRecipient>(recipientPath(id), {
      method: "PUT",
      body: recipientBody(draft),
    }),
  );
}

/** Removes a recipient and its password; its formats go to the download. */
export async function deleteExportRecipient(
  id: string,
): Promise<ExportRecipientsPage> {
  return recipientsPageFromBackend(
    await traegerRequest<BackendRecipientsPage>(recipientPath(id), {
      method: "DELETE",
    }),
  );
}

/** Sends each format to its recipient, or to the download for null. */
export async function assignExportFormats(
  assignments: readonly ExportFormatAssignment[],
): Promise<ExportRecipientsPage> {
  return recipientsPageFromBackend(
    await traegerRequest<BackendRecipientsPage>(`${RECIPIENTS}/formats`, {
      method: "PUT",
      body: {
        assignments: assignments.map((assignment) => ({
          format: assignment.format,
          recipient_id:
            assignment.recipientId === null
              ? null
              : Number(assignment.recipientId),
        })),
      },
    }),
  );
}

/** Makes the file of a run and hands it to the recipient. */
export async function handOverRunFile(
  format: HandoverFormat,
  pathKind: ExportRunPathKind,
  runId: string,
  options: HandoverOptions = {},
): Promise<HandoverResult> {
  const result = await traegerRequest<BackendResult>(
    `${HANDOVERS}/runs/${pathKind}/${encodeURIComponent(runId)}/${format}`,
    {
      method: "POST",
      body: {
        repeat: options.repeat ?? false,
        ...(options.all ? { mode: "all" } : {}),
        ...(options.fiscalYear ? { fiscal_year: options.fiscalYear } : {}),
      },
    },
  );
  return {
    fileName: result.file_name,
    state: result.state,
    reason: result.reason ?? "",
    recipientName: result.recipient_name ?? "",
    targetHost: result.target_host,
    targetDirectory: result.target_directory,
  };
}

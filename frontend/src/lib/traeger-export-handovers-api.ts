/**
 * Client of the handover of finance exports to the recipient of the Träger
 * (#6824, ADR 0218): the recipient without its password, its newest
 * handovers, and the handover of a Sollstellung, a DATEV booking batch or a
 * PAIN.008 file. Backend int64 IDs leave here as strings, instants as ISO
 * strings.
 */

import type {
  ExportFormat,
  ExportRunPathKind,
} from "./traeger-finance-exports-api";
import { traegerRequest } from "./traeger-api";

const RECIPIENT = "/api/traeger/export-recipient";
const HANDOVERS = "/api/traeger/finance-handovers";

/** The fields of a recipient, in form order; the backend names these. */
export type RecipientField =
  | "host"
  | "port"
  | "username"
  | "password"
  | "remote_directory"
  | "host_key_fingerprint";

export interface ExportRecipient {
  /** Whether the Träger saved a recipient at all. */
  readonly configured: boolean;
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
  readonly updatedAt: string | null;
}

/** The outcome of a handover, as the journal and the protocol name it. */
export type HandoverState =
  "transferred" | "failed" | "in_progress" | "unknown";

export interface ExportHandover {
  readonly id: string;
  readonly createdAt: string;
  readonly format: ExportFormat;
  readonly fileName: string;
  readonly byteSize: number;
  readonly targetHost: string;
  readonly targetDirectory: string;
  readonly state: HandoverState;
  /** Stable reason code of a failure, empty otherwise. */
  readonly reason: string;
}

export interface ExportRecipientPage {
  readonly recipient: ExportRecipient;
  readonly handovers: readonly ExportHandover[];
}

/** What the Träger saves. Without a password the stored one stays. */
export interface ExportRecipientDraft {
  readonly enabled: boolean;
  readonly host: string;
  readonly port: number;
  readonly username: string;
  readonly password?: string;
  readonly remoteDirectory: string;
  readonly hostKeyFingerprint: string;
}

export type HandoverFormat = Extract<
  ExportFormat,
  "sollstellung" | "datev" | "pain008"
>;

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
  readonly targetHost: string;
  readonly targetDirectory: string;
}

interface BackendRecipient {
  configured: boolean;
  enabled: boolean;
  ready: boolean;
  host: string;
  port: number;
  username: string;
  remote_directory: string;
  host_key_fingerprint: string;
  password_set: boolean;
  missing_fields: RecipientField[] | null;
  updated_at?: string | null;
}

interface BackendHandover {
  id: number;
  created_at: string;
  format: ExportFormat;
  filename: string;
  byte_size: number;
  target_host: string;
  target_directory: string;
  state: HandoverState;
  reason?: string;
}

interface BackendResult {
  file_name: string;
  state: HandoverState;
  reason?: string;
  target_host: string;
  target_directory: string;
}

function recipientFromBackend(recipient: BackendRecipient): ExportRecipient {
  return {
    configured: recipient.configured,
    enabled: recipient.enabled,
    ready: recipient.ready,
    host: recipient.host,
    port: recipient.port,
    username: recipient.username,
    remoteDirectory: recipient.remote_directory,
    hostKeyFingerprint: recipient.host_key_fingerprint,
    passwordSet: recipient.password_set,
    missingFields: recipient.missing_fields ?? [],
    updatedAt: recipient.updated_at ?? null,
  };
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
    state: handover.state,
    reason: handover.reason ?? "",
  };
}

/** The recipient of the Träger with its newest handovers. */
export async function fetchExportRecipient(): Promise<ExportRecipientPage> {
  const page = await traegerRequest<{
    recipient: BackendRecipient;
    handovers: BackendHandover[] | null;
  }>(RECIPIENT);
  return {
    recipient: recipientFromBackend(page.recipient),
    handovers: (page.handovers ?? []).map(handoverFromBackend),
  };
}

/** Saves the recipient; the backend names each wrong field. */
export async function saveExportRecipient(
  draft: ExportRecipientDraft,
): Promise<ExportRecipient> {
  return recipientFromBackend(
    await traegerRequest<BackendRecipient>(RECIPIENT, {
      method: "PUT",
      body: {
        enabled: draft.enabled,
        host: draft.host,
        port: draft.port,
        username: draft.username,
        ...(draft.password ? { password: draft.password } : {}),
        remote_directory: draft.remoteDirectory,
        host_key_fingerprint: draft.hostKeyFingerprint,
      },
    }),
  );
}

/** Removes the recipient and its stored password. */
export async function deleteExportRecipient(): Promise<ExportRecipient> {
  return recipientFromBackend(
    await traegerRequest<BackendRecipient>(RECIPIENT, { method: "DELETE" }),
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
    targetHost: result.target_host,
    targetDirectory: result.target_directory,
  };
}

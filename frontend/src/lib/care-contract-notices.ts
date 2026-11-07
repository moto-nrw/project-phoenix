/**
 * Withdrawal and termination of a care contract (#4682): what a contract
 * allows today and the declaration made, for the Träger-Büro and for the
 * parents' button „Verträge hier kündigen“ (§ 312k BGB). Backend int64 IDs
 * leave here as strings; calendar dates stay "YYYY-MM-DD". Types and mapping
 * only; the Träger client lives in care-contract-notices-api.ts.
 */

export type NoticeKind = "kuendigung" | "widerruf";
export type TerminationKind = "ordentlich" | "sonderkuendigung";

/** A declaration as recorded, with its confirmation as sent. */
export interface ContractNotice {
  readonly kind: NoticeKind;
  readonly terminationKind: TerminationKind | null;
  /** The parents with the button, or the office for a received letter. */
  readonly channel: "eltern_app" | "traeger_buero";
  readonly declaredAt: string;
  readonly receivedOn: string;
  readonly dispatchedOn?: string | null;
  /** Last day of a terminated contract; null after a withdrawal. */
  readonly endsOn: string | null;
  readonly recipient: string;
  readonly confirmationText: string;
}

/** What can be declared on a contract today, or the declaration made. */
export interface ContractNoticeOptions {
  readonly contractId: string;
  readonly traegerName: string;
  readonly status: string;
  readonly childName: string;
  readonly modules: string;
  readonly schoolName: string;
  /** The notice terms as the contract PDF says them. */
  readonly terms: string;
  readonly ordinaryEndsOn: string | null;
  /** Earlier end of the special termination; null when there is none. */
  readonly specialEndsOn: string | null;
  /** Last dispatch day. Office reads retain expired deadlines for postal notices. */
  readonly withdrawalUntil: string | null;
  readonly earlyStart: boolean;
  readonly notice: ContractNotice | null;
}

export interface NoticeDeclaration {
  readonly kind: NoticeKind;
  readonly terminationKind?: TerminationKind;
  readonly expectedEndsOn?: string;
}

interface BackendNotice {
  kind: NoticeKind;
  termination_kind?: TerminationKind;
  channel: "eltern_app" | "traeger_buero";
  declared_at: string;
  received_on: string;
  dispatched_on?: string | null;
  ends_on: string | null;
  recipient: string;
  confirmation_text: string;
}

export interface BackendNoticeOptions {
  contract_id: number | string;
  traeger_name?: string;
  status: string;
  child_name: string;
  modules: string;
  school_name: string;
  terms: string;
  ordinary_ends_on: string | null;
  special_ends_on: string | null;
  withdrawal_until: string | null;
  early_start: boolean;
  notice: BackendNotice | null;
}

export function noticeOptionsFromBackend(
  row: BackendNoticeOptions,
): ContractNoticeOptions {
  return {
    contractId: String(row.contract_id),
    traegerName: row.traeger_name ?? "",
    status: row.status,
    childName: row.child_name,
    modules: row.modules,
    schoolName: row.school_name,
    terms: row.terms,
    ordinaryEndsOn: row.ordinary_ends_on,
    specialEndsOn: row.special_ends_on,
    withdrawalUntil: row.withdrawal_until,
    earlyStart: row.early_start,
    notice: row.notice
      ? {
          kind: row.notice.kind,
          terminationKind: row.notice.termination_kind ?? null,
          channel: row.notice.channel,
          declaredAt: row.notice.declared_at,
          receivedOn: row.notice.received_on,
          dispatchedOn: row.notice.dispatched_on ?? null,
          endsOn: row.notice.ends_on,
          recipient: row.notice.recipient,
          confirmationText: row.notice.confirmation_text,
        }
      : null,
  };
}

export function declarationBody(declaration: NoticeDeclaration) {
  return {
    kind: declaration.kind,
    ...(declaration.kind === "kuendigung"
      ? {
          termination_kind: declaration.terminationKind ?? "ordentlich",
          ...(declaration.expectedEndsOn
            ? { expected_ends_on: declaration.expectedEndsOn }
            : {}),
        }
      : {}),
  };
}

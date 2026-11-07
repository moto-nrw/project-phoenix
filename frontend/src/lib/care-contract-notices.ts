/**
 * Withdrawal and termination of a care contract (#4682): what a contract
 * allows today and the declaration made, for the Träger-Büro and for the
 * parents' button „Verträge hier kündigen“ (§ 312k BGB). Backend int64 IDs
 * leave here as strings; calendar dates stay "YYYY-MM-DD". Types and mapping
 * only; the Träger client lives in care-contract-notices-api.ts.
 */

export type NoticeKind = "kuendigung" | "widerruf";
/**
 * How the parents terminate (§ 312k Abs. 2 BGB): to the next date, to the
 * special date, or extraordinarily for a reason they name. The Träger sets
 * the end of an extraordinary termination after it checked the reason.
 */
export type TerminationKind =
  "ordentlich" | "sonderkuendigung" | "ausserordentlich";

/**
 * How the Träger-Büro decided the end of an extraordinary termination: it
 * set the last day, or it did not accept the reason and the declaration
 * holds as an ordinary termination (Umdeutung, § 140 BGB).
 */
export type EndDecision = "festgelegt" | "umgedeutet";

/** A declaration as recorded, with its confirmation as sent. */
export interface ContractNotice {
  readonly kind: NoticeKind;
  readonly terminationKind: TerminationKind | null;
  /** The parents' reason of an extraordinary termination, else null. */
  readonly reason: string | null;
  /** The parents with the button, or the office for a received letter. */
  readonly channel: "eltern_app" | "traeger_buero";
  readonly declaredAt: string;
  readonly receivedOn: string;
  readonly dispatchedOn?: string | null;
  /**
   * Last day of a terminated contract; null after a withdrawal and while
   * the Träger checks an extraordinary termination.
   */
  readonly endsOn: string | null;
  readonly recipient: string;
  readonly confirmationText: string;
  /** Null while the end is open and for every other declaration. */
  readonly endDecision?: EndDecision | null;
  readonly endDecidedAt?: string | null;
  /** The notice of the decided end the parents got, as sent. */
  readonly endNoticeText?: string | null;
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
  /**
   * For the office, while an extraordinary termination waits: the last day
   * if it does not accept the reason. Null otherwise.
   */
  readonly reinterpretedEndsOn?: string | null;
  readonly notice: ContractNotice | null;
}

export interface NoticeDeclaration {
  readonly kind: NoticeKind;
  readonly terminationKind?: TerminationKind;
  readonly expectedEndsOn?: string;
  /** Required with an extraordinary termination, never sent otherwise. */
  readonly reason?: string;
}

interface BackendNotice {
  kind: NoticeKind;
  termination_kind?: TerminationKind;
  reason?: string;
  channel: "eltern_app" | "traeger_buero";
  declared_at: string;
  received_on: string;
  dispatched_on?: string | null;
  ends_on: string | null;
  recipient: string;
  confirmation_text: string;
  end_decision?: EndDecision;
  end_decided_at?: string;
  end_notice_text?: string;
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
  reinterpreted_ends_on?: string | null;
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
    reinterpretedEndsOn: row.reinterpreted_ends_on ?? null,
    notice: row.notice
      ? {
          kind: row.notice.kind,
          terminationKind: row.notice.termination_kind ?? null,
          reason: row.notice.reason ?? null,
          channel: row.notice.channel,
          declaredAt: row.notice.declared_at,
          receivedOn: row.notice.received_on,
          dispatchedOn: row.notice.dispatched_on ?? null,
          endsOn: row.notice.ends_on,
          recipient: row.notice.recipient,
          confirmationText: row.notice.confirmation_text,
          endDecision: row.notice.end_decision ?? null,
          endDecidedAt: row.notice.end_decided_at ?? null,
          endNoticeText: row.notice.end_notice_text ?? null,
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
          ...(declaration.terminationKind === "ausserordentlich"
            ? { reason: declaration.reason ?? "" }
            : {}),
        }
      : {}),
  };
}

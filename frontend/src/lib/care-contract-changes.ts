/**
 * The parents' change of the booked care of a contract's child, as the
 * Träger-Büro decides it (#5979): the open request with its modules and
 * days, and the last decision. Backend int64 IDs leave here as strings;
 * calendar dates stay "YYYY-MM-DD". Types and mapping only; the client lives
 * in care-contract-changes-api.ts.
 */

import type { Weekday } from "./traeger-care-contracts-api";

export type ChangeModuleState = "added" | "removed" | "changed" | "unchanged";

/** One module of a change: what the child books now and what is asked. */
export interface ContractChangeLine {
  readonly careOfferingId: string;
  readonly moduleName: string;
  readonly state: ChangeModuleState;
  readonly beforeDays: readonly Weekday[];
  readonly afterDays: readonly Weekday[];
}

/** A change the parents asked for that nobody decided yet. */
export interface ContractChangeRequest {
  readonly id: string;
  /** Pins the request the decision is taken on. */
  readonly version: string;
  readonly submittedAt: string;
  readonly effectiveFrom: string;
  readonly parentNote: string;
  /** The parents give up every module: that is a termination. */
  readonly endsAllCare: boolean;
  readonly lines: readonly ContractChangeLine[];
}

/** The newest decided change of the child. */
export interface ContractChangeOutcome {
  readonly approved: boolean;
  readonly decidedAt: string;
  readonly effectiveFrom: string;
  readonly reason: string;
}

export interface ContractChanges {
  /** The office may decide: the contract is active. */
  readonly decidable: boolean;
  readonly pending: readonly ContractChangeRequest[];
  readonly lastDecision: ContractChangeOutcome | null;
  /** The school asks for a reason on an approval too. */
  readonly reasonRequired: boolean;
}

export interface ContractChangeDecision {
  readonly approve: boolean;
  readonly reason: string;
  /** The day the change takes effect; only with an approval. */
  readonly effectiveFrom?: string;
  readonly expectedVersion: string;
}

interface BackendChangeLine {
  care_offering_id: number | string;
  module_name: string;
  state: ChangeModuleState;
  before_days: Weekday[] | null;
  after_days: Weekday[] | null;
}

interface BackendChangeRequest {
  id: number | string;
  version: string;
  submitted_at: string;
  effective_from: string;
  parent_note: string;
  ends_all_care: boolean;
  lines: BackendChangeLine[] | null;
}

export interface BackendContractChanges {
  decidable: boolean;
  pending: BackendChangeRequest[] | null;
  last_decision: {
    approved: boolean;
    decided_at: string;
    effective_from: string;
    reason: string;
  } | null;
  reason_required: boolean;
}

export function contractChangesFromBackend(
  data: BackendContractChanges,
): ContractChanges {
  return {
    decidable: data.decidable,
    reasonRequired: data.reason_required,
    pending: (data.pending ?? []).map((request) => ({
      id: String(request.id),
      version: request.version,
      submittedAt: request.submitted_at,
      effectiveFrom: request.effective_from,
      parentNote: request.parent_note,
      endsAllCare: request.ends_all_care,
      lines: (request.lines ?? []).map((line) => ({
        careOfferingId: String(line.care_offering_id),
        moduleName: line.module_name,
        state: line.state,
        beforeDays: line.before_days ?? [],
        afterDays: line.after_days ?? [],
      })),
    })),
    lastDecision: data.last_decision
      ? {
          approved: data.last_decision.approved,
          decidedAt: data.last_decision.decided_at,
          effectiveFrom: data.last_decision.effective_from,
          reason: data.last_decision.reason,
        }
      : null,
  };
}

export function decisionBody(decision: ContractChangeDecision) {
  return {
    approve: decision.approve,
    reason: decision.reason.trim(),
    effective_from: decision.approve ? (decision.effectiveFrom ?? null) : null,
    expected_version: decision.expectedVersion,
  };
}

/** What happens to one module on one weekday with the change. */
export type ChangeDayState = "kept" | "added" | "removed" | "none";

export interface ChangeWeekRow {
  readonly careOfferingId: string;
  readonly moduleName: string;
  readonly days: readonly {
    readonly weekday: Weekday;
    readonly state: ChangeDayState;
  }[];
}

const WEEK: readonly Weekday[] = [
  "mon",
  "tue",
  "wed",
  "thu",
  "fri",
  "sat",
  "sun",
];

/**
 * The week of a change, module by module: Monday to Friday always, the
 * weekend only when a module books it before or after.
 */
export function changeWeek(lines: readonly ContractChangeLine[]): {
  readonly weekdays: readonly Weekday[];
  readonly rows: readonly ChangeWeekRow[];
} {
  const weekend = new Set(
    lines.flatMap((line) => [...line.beforeDays, ...line.afterDays]),
  );
  const weekdays = WEEK.filter(
    (weekday, index) => index < 5 || weekend.has(weekday),
  );
  const rows = lines.map((line) => ({
    careOfferingId: line.careOfferingId,
    moduleName: line.moduleName,
    days: weekdays.map((weekday) => {
      const before = line.beforeDays.includes(weekday);
      const after = line.afterDays.includes(weekday);
      const state: ChangeDayState =
        before && after
          ? "kept"
          : after
            ? "added"
            : before
              ? "removed"
              : "none";
      return { weekday, state };
    }),
  }));
  return { weekdays, rows };
}

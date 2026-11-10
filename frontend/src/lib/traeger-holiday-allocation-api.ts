/**
 * Client of the allocation by criteria in the Träger portal (#4700): the
 * criteria of the Träger, the allocation list of a holiday period and the
 * decisions of the Träger-Büro. moto only proposes an order; every place is
 * given by a person of the office. Talks to the BFF routes under
 * /api/traeger/holiday-periods. Backend int64 IDs leave here as strings.
 */

import { traegerRequest } from "./traeger-api";

/** Whether a criterion is asked once per family or for every child. */
export type HolidayCriterionScope = "family" | "child";

/** What the allocation decides for one booking. */
export type HolidayAllocationPlacement = "placed" | "waitlisted";

/** One criterion of the Träger, in the order of its priority. */
export interface HolidayCriterion {
  readonly id: string;
  readonly name: string;
  /** The yes/no question the parents answer. */
  readonly question: string;
  readonly scope: HolidayCriterionScope;
  readonly active: boolean;
  /** 1 is the highest priority. */
  readonly position: number;
  /** Parents answered it already: removing it archives it. */
  readonly answered: boolean;
}

/** A criterion as the editor saves it; an empty ID creates it. */
export interface HolidayCriterionInput {
  readonly id: string;
  readonly name: string;
  readonly question: string;
  readonly scope: HolidayCriterionScope;
  readonly active: boolean;
}

/** The backend's limits of the criteria. */
export const MAX_HOLIDAY_CRITERIA = 10;
export const MAX_CRITERION_NAME = 60;
export const MAX_CRITERION_QUESTION = 300;
export const MAX_ALLOCATION_REASON = 500;

interface BackendCriterion {
  id: number;
  name: string;
  question: string;
  scope: HolidayCriterionScope;
  active: boolean;
  position: number;
  answered?: boolean;
}

function criteriaFromBackend(
  data: { criteria?: BackendCriterion[] | null } | null,
): HolidayCriterion[] {
  return (data?.criteria ?? []).map((criterion) => ({
    id: String(criterion.id),
    name: criterion.name,
    question: criterion.question,
    scope: criterion.scope,
    active: criterion.active,
    position: criterion.position,
    answered: criterion.answered ?? false,
  }));
}

const criteriaPath = "/api/traeger/holiday-periods/criteria";

/** The criteria of the Träger, highest priority first. */
export async function fetchHolidayCriteria(): Promise<HolidayCriterion[]> {
  return criteriaFromBackend(
    await traegerRequest<{ criteria?: BackendCriterion[] | null }>(
      criteriaPath,
    ),
  );
}

/**
 * Saves the whole list; its order is the priority. A criterion left out is
 * archived: given answers and decisions keep it.
 */
export async function saveHolidayCriteria(
  criteria: readonly HolidayCriterionInput[],
): Promise<HolidayCriterion[]> {
  return criteriaFromBackend(
    await traegerRequest<{ criteria?: BackendCriterion[] | null }>(
      criteriaPath,
      {
        method: "PUT",
        body: {
          criteria: criteria.map((criterion) => ({
            id: criterion.id ? Number(criterion.id) : 0,
            name: criterion.name,
            question: criterion.question,
            scope: criterion.scope,
            active: criterion.active,
          })),
        },
      },
    ),
  );
}

/** "yes" met, "no" not met, "unknown" without an answer. */
export type HolidayCriterionValue = "yes" | "no" | "unknown";

/** The value of one criterion for an open booking. */
interface HolidayAllocationCriterionValue {
  readonly criterionId: string;
  readonly value: HolidayCriterionValue;
  /** "office": the Träger-Büro checked it; "parent": the parents said so. */
  readonly source: "parent" | "office";
}

/** One open booking of a week, in the order of the proposal. */
export interface HolidayAllocationEntry {
  readonly bookingId: string;
  /** Place in the proposal of the week, from 1. */
  readonly rank: number;
  readonly registrationId: string;
  readonly submittedAt: string;
  readonly childId: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly timeModelName: string;
  /** The wanted days of a week by days; empty for a whole week. */
  readonly days: readonly string[];
  /** In the order of the priority. */
  readonly criteria: readonly HolidayAllocationCriterionValue[];
  readonly proposal: HolidayAllocationPlacement;
}

export interface HolidayAllocationWeek {
  readonly weekId: string;
  /** Calendar dates "YYYY-MM-DD". */
  readonly startsOn: string;
  readonly endsOn: string;
  readonly hostSchoolId: string;
  readonly bookingUnit: "week" | "day";
  /** Null is unlimited. */
  readonly capacity: number | null;
  readonly placed: number;
  /** Free places; the smallest of the days for a week by days. Null is unlimited. */
  readonly free: number | null;
  readonly pending: readonly HolidayAllocationEntry[];
}

/** One decision of the office, kept for the record. */
export interface HolidayAllocationDecision {
  readonly id: string;
  readonly weekId: string;
  readonly weekStartsOn: string;
  readonly weekEndsOn: string;
  readonly registrationId: string;
  readonly childId: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly placement: HolidayAllocationPlacement;
  readonly proposal: HolidayAllocationPlacement;
  readonly rank: number;
  /** The names of the criteria met at the time of the decision. */
  readonly criteriaMet: readonly string[];
  /** Required when the decision differs from the proposal. */
  readonly reason: string;
  readonly decidedAt: string;
}

/** An active criterion as the allocation list names it. */
export interface HolidayAllocationCriterion {
  readonly id: string;
  readonly name: string;
  readonly scope: HolidayCriterionScope;
  readonly position: number;
}

export interface HolidayAllocationBoard {
  readonly criteria: readonly HolidayAllocationCriterion[];
  readonly weeks: readonly HolidayAllocationWeek[];
  /** Newest first. */
  readonly decisions: readonly HolidayAllocationDecision[];
}

/** One decision the office sends. */
export interface HolidayAllocationDecisionInput {
  readonly bookingId: string;
  readonly placement: HolidayAllocationPlacement;
  /** The proposal the office saw; the backend refuses a stale one. */
  readonly proposal: HolidayAllocationPlacement;
  readonly reason: string;
}

interface BackendBoard {
  criteria?:
    | {
        id: number;
        name: string;
        scope: HolidayCriterionScope;
        position: number;
      }[]
    | null;
  weeks?:
    | {
        week_id: number;
        starts_on: string;
        ends_on: string;
        host_school_id: number;
        booking_unit?: "week" | "day";
        capacity: number | null;
        placed: number;
        free: number | null;
        pending?:
          | {
              booking_id: number;
              rank: number;
              registration_id: number;
              submitted_at: string;
              child_id: number;
              child_first_name: string;
              child_last_name: string;
              time_model_name: string;
              days?: string[] | null;
              criteria?:
                | {
                    criterion_id: number;
                    value: HolidayCriterionValue;
                    source: "parent" | "office";
                  }[]
                | null;
              proposal: HolidayAllocationPlacement;
            }[]
          | null;
      }[]
    | null;
  decisions?:
    | {
        id: number;
        week_id: number;
        week_starts_on: string;
        week_ends_on: string;
        registration_id: number;
        child_id: number;
        child_first_name: string;
        child_last_name: string;
        placement: HolidayAllocationPlacement;
        proposal: HolidayAllocationPlacement;
        rank: number;
        criteria_met?: string[] | null;
        reason?: string | null;
        decided_at: string;
      }[]
    | null;
}

function boardFromBackend(board: BackendBoard | null): HolidayAllocationBoard {
  return {
    criteria: (board?.criteria ?? []).map((criterion) => ({
      id: String(criterion.id),
      name: criterion.name,
      scope: criterion.scope,
      position: criterion.position,
    })),
    weeks: (board?.weeks ?? []).map((week) => ({
      weekId: String(week.week_id),
      startsOn: week.starts_on,
      endsOn: week.ends_on,
      hostSchoolId: String(week.host_school_id),
      bookingUnit: week.booking_unit ?? "week",
      capacity: week.capacity,
      placed: week.placed,
      free: week.free,
      pending: (week.pending ?? []).map((entry) => ({
        bookingId: String(entry.booking_id),
        rank: entry.rank,
        registrationId: String(entry.registration_id),
        submittedAt: entry.submitted_at,
        childId: String(entry.child_id),
        childFirstName: entry.child_first_name,
        childLastName: entry.child_last_name,
        timeModelName: entry.time_model_name,
        days: entry.days ?? [],
        criteria: (entry.criteria ?? []).map((value) => ({
          criterionId: String(value.criterion_id),
          value: value.value,
          source: value.source,
        })),
        proposal: entry.proposal,
      })),
    })),
    decisions: (board?.decisions ?? []).map((decision) => ({
      id: String(decision.id),
      weekId: String(decision.week_id),
      weekStartsOn: decision.week_starts_on,
      weekEndsOn: decision.week_ends_on,
      registrationId: String(decision.registration_id),
      childId: String(decision.child_id),
      childFirstName: decision.child_first_name,
      childLastName: decision.child_last_name,
      placement: decision.placement,
      proposal: decision.proposal,
      rank: decision.rank,
      criteriaMet: decision.criteria_met ?? [],
      reason: decision.reason ?? "",
      decidedAt: decision.decided_at,
    })),
  };
}

const allocationPath = (periodId: string) =>
  `/api/traeger/holiday-periods/${encodeURIComponent(periodId)}/allocation`;

/** The open bookings of every week with the proposal, and the decisions. */
export async function fetchHolidayAllocation(
  periodId: string,
): Promise<HolidayAllocationBoard> {
  return boardFromBackend(
    await traegerRequest<BackendBoard>(allocationPath(periodId)),
  );
}

/**
 * Decides open bookings and answers with the new list. Parents get an
 * e-mail per registration.
 */
export async function decideHolidayAllocation(
  periodId: string,
  decisions: readonly HolidayAllocationDecisionInput[],
): Promise<HolidayAllocationBoard> {
  return boardFromBackend(
    await traegerRequest<BackendBoard>(
      `${allocationPath(periodId)}/decisions`,
      {
        method: "POST",
        body: {
          decisions: decisions.map((decision) => ({
            booking_id: Number(decision.bookingId),
            placement: decision.placement,
            proposal: decision.proposal,
            reason: decision.reason,
          })),
        },
      },
    ),
  );
}

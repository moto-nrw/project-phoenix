// Länger als gebucht (#4683, ADR 0154): wie oft Kinder an den Schulen des
// Trägers länger da waren, als ihr Vertrag bucht. Ohne Namen braucht die
// Auswertung das Recht „Berichte lesen“, mit Namen „Verträge verwalten“.

import { traegerRequest } from "./traeger-api";

const OVERRUNS_PATH = "/api/traeger/care-time-overruns";

/** Das Recht, das die Kinder mit Namen öffnet. */
export const OVERRUN_NAMES_RIGHT = "traeger_contracts:manage";

interface OverrunFigures {
  readonly children: number;
  readonly overrunDays: number;
  readonly unbookedDays: number;
  readonly extraMinutes: number;
}

export interface SchoolOverruns {
  readonly schoolId: string;
  readonly schoolName: string;
  /** Erster verglichener Tag; die Löschfrist der Check-ins kann ihn kürzen. */
  readonly dataFrom: string;
  readonly figures: OverrunFigures;
}

export interface OverrunChild {
  readonly schoolId: string;
  readonly schoolName: string;
  readonly studentId: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly schoolClass: string;
  readonly overrunDays: number;
  readonly unbookedDays: number;
  readonly extraMinutes: number;
}

export interface OverrunEvaluation {
  readonly from: string;
  readonly to: string;
  readonly toleranceMinutes: number;
  readonly total: OverrunFigures;
  readonly schools: readonly SchoolOverruns[];
  readonly children: readonly OverrunChild[];
}

interface BackendFigures {
  children: number;
  overrun_days: number;
  unbooked_days: number;
  extra_minutes: number;
}

interface BackendEvaluation {
  from: string;
  to: string;
  tolerance_minutes: number;
  total: BackendFigures;
  schools:
    | {
        school_id: number | string;
        school_name: string;
        data_from: string;
        figures: BackendFigures;
      }[]
    | null;
  children:
    | {
        school_id: number | string;
        school_name: string;
        student_id: number | string;
        first_name: string;
        last_name: string;
        school_class: string;
        overrun_days: number;
        unbooked_days: number;
        extra_minutes: number;
      }[]
    | null;
}

function figuresFromBackend(
  figures: BackendFigures | undefined,
): OverrunFigures {
  return {
    children: figures?.children ?? 0,
    overrunDays: figures?.overrun_days ?? 0,
    unbookedDays: figures?.unbooked_days ?? 0,
    extraMinutes: figures?.extra_minutes ?? 0,
  };
}

function evaluationFromBackend(body: BackendEvaluation): OverrunEvaluation {
  return {
    from: body.from,
    to: body.to,
    toleranceMinutes: body.tolerance_minutes,
    total: figuresFromBackend(body.total),
    schools: (body.schools ?? []).map((school) => ({
      schoolId: String(school.school_id),
      schoolName: school.school_name,
      dataFrom: school.data_from,
      figures: figuresFromBackend(school.figures),
    })),
    children: (body.children ?? []).map((child) => ({
      schoolId: String(child.school_id),
      schoolName: child.school_name,
      studentId: String(child.student_id),
      firstName: child.first_name,
      lastName: child.last_name,
      schoolClass: child.school_class,
      overrunDays: child.overrun_days,
      unbookedDays: child.unbooked_days,
      extraMinutes: child.extra_minutes,
    })),
  };
}

export interface OverrunWindow {
  readonly from: string;
  readonly to: string;
  readonly schoolId?: string;
}

function overrunQuery(window: OverrunWindow): string {
  const query = new URLSearchParams({ from: window.from, to: window.to });
  if (window.schoolId) query.set("school_id", window.schoolId);
  return query.toString();
}

/** Die Zahlen je Schule, ohne Namen. */
export async function fetchOverrunSummary(
  window: OverrunWindow,
): Promise<OverrunEvaluation> {
  return evaluationFromBackend(
    await traegerRequest<BackendEvaluation>(
      `${OVERRUNS_PATH}?${overrunQuery(window)}`,
    ),
  );
}

/** Die Zahlen je Schule und die Kinder mit Namen; steht im Zugriffsprotokoll. */
export async function fetchOverrunChildren(
  window: OverrunWindow,
): Promise<OverrunEvaluation> {
  return evaluationFromBackend(
    await traegerRequest<BackendEvaluation>(
      `${OVERRUNS_PATH}/children?${overrunQuery(window)}`,
    ),
  );
}

/** „2 Std. 15 Min.“, „45 Min.“, „0 Min.“ */
export function formatExtraTime(minutes: number): string {
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  if (hours === 0) return `${rest} Min.`;
  if (rest === 0) return `${hours} Std.`;
  return `${hours} Std. ${rest} Min.`;
}

/** Tage mit längerer Betreuung, ob gebucht oder nicht. */
export function overrunDayCount(figures: {
  readonly overrunDays: number;
  readonly unbookedDays: number;
}): number {
  return figures.overrunDays + figures.unbookedDays;
}

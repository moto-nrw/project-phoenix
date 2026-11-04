// Tagesübersicht der ganzen Schule (#4802): welche Zahlen die Startseite der
// Schulverwaltung zeigt und welche Kinder hinter jeder Zahl stehen.
//
// Jede Zahl ist ein Link auf dieselbe Seite mit `?liste=…`; darunter stehen
// dann die Kinder dieser Zahl, nach Klasse gruppiert. Die Liste steht in der
// Adresse, damit der Zurück-Weg aus einer Klasse auf dieselbe Liste führt.
//
// Diese Datei enthält nur die Regeln, kein React, kein Fetch.

import type {
  SchoolDayChild,
  SchoolDayReport,
  SchoolDayTotals,
} from "~/lib/school-class-day-api";
import { classDayOverviewPath, classDayPath } from "./routes";

/** Query-Parameter, unter dem die geöffnete Liste in der Adresse steht. */
const SCHOOL_DAY_LIST_PARAM = "liste";

export type SchoolDayListKey =
  | "kinder"
  | "fehlen"
  | "ohne-meldung"
  | "krank"
  | "entschuldigt"
  | "klassenfahrt"
  | "bleiben"
  | "gehen-heim";

export interface SchoolDayList {
  readonly key: SchoolDayListKey;
  readonly label: string;
  readonly count: (totals: SchoolDayTotals) => number;
  readonly matches: (child: SchoolDayChild) => boolean;
  /** Nur zeigen, wenn eine Anmeldephase den Tag abdeckt. */
  readonly needsEnrollment?: boolean;
}

/**
 * Die Zahlen in der Reihenfolge der Frage „Wer fehlt heute, wer ist krank,
 * wer geht in die OGS“. Die Wörter sind die der Klassenansicht
 * (status-labels.ts, Klassenzeile), damit eine Zahl hier und dort gleich
 * heißt.
 */
const SCHOOL_DAY_LISTS: readonly SchoolDayList[] = [
  {
    key: "kinder",
    label: "Kinder",
    count: (totals) => totals.students,
    matches: () => true,
  },
  {
    key: "fehlen",
    label: "Fehlen",
    count: (totals) => totals.missing,
    matches: (child) => child.missing === true,
  },
  {
    // Die Lehrkraft hat das Kind als fehlend eingetragen, und niemand hat
    // es gemeldet (#4791). Hier trägt die Schulverwaltung die Meldung nach.
    key: "ohne-meldung",
    label: "Fehlen ohne Meldung",
    count: (totals) => totals.unreported ?? 0,
    matches: (child) => child.unreported === true,
  },
  {
    key: "krank",
    label: "Krank",
    count: (totals) => totals.sick,
    matches: (child) => child.status === "sick",
  },
  {
    key: "entschuldigt",
    label: "Entschuldigt",
    count: (totals) => totals.excused,
    matches: (child) => child.status === "excused",
  },
  {
    key: "klassenfahrt",
    label: "Klassenfahrt",
    count: (totals) => totals.class_trip,
    matches: (child) => child.status === "class_trip",
  },
  {
    key: "bleiben",
    label: "Bleiben in der OGS",
    count: (totals) => totals.staying,
    matches: (child) => child.after_school === "stays",
    needsEnrollment: true,
  },
  {
    key: "gehen-heim",
    label: "Gehen heim",
    count: (totals) => totals.leaving,
    matches: (child) => child.after_school === "goes_home",
    needsEnrollment: true,
  },
];

/**
 * Die Zahlen, die der Tag tragen kann. Ohne Anmeldephase ist offen, wer
 * bleibt; eine 0 bei „Bleiben in der OGS“ wäre dann eine falsche Aussage.
 */
export function visibleSchoolDayLists(
  report: SchoolDayReport,
): readonly SchoolDayList[] {
  return SCHOOL_DAY_LISTS.filter(
    (list) => !list.needsEnrollment || report.enrollment_known,
  );
}

/** Liste aus der Adresse lesen; Unbekanntes öffnet keine Liste. */
export function schoolDayListParam(
  raw: string | null | undefined,
): SchoolDayListKey | null {
  return SCHOOL_DAY_LISTS.find((list) => list.key === raw)?.key ?? null;
}

/** Adresse der Übersicht eines Tages, mit oder ohne geöffnete Liste. */
export function schoolDayListPath(
  dateISO: string,
  list: SchoolDayListKey | null,
): string {
  const base = classDayOverviewPath(dateISO);
  return list ? `${base}&${SCHOOL_DAY_LIST_PARAM}=${list}` : base;
}

/** Adresse einer Klasse mit der geöffneten Liste für den Rückweg. */
export function schoolDayClassPath(
  schoolClass: string,
  dateISO: string,
  list: SchoolDayListKey | null,
): string {
  const base = classDayPath(schoolClass, dateISO);
  return list ? `${base}&${SCHOOL_DAY_LIST_PARAM}=${list}` : base;
}

export interface SchoolDayClassGroup {
  readonly schoolClass: string;
  readonly children: readonly SchoolDayChild[];
}

/** Die Kinder einer Liste, nach Klasse gruppiert, ohne leere Klassen. */
export function schoolDayChildrenByClass(
  report: SchoolDayReport,
  list: SchoolDayList,
): SchoolDayClassGroup[] {
  return report.classes
    .map((klass) => ({
      schoolClass: klass.school_class,
      children: klass.children.filter(list.matches),
    }))
    .filter((group) => group.children.length > 0);
}

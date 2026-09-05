"use client";

/**
 * PlanungRedirect — Client-Redirect der alten Planungs-Routen.
 *
 * Die frühere Tab-Seite /planung (#1886) und die Alt-Stubs /timetables,
 * /vertretungsplan und /staff/dienstplan leiten auf die drei eigenständigen
 * Bereiche /betreuungsplan, /dienstplan und /vertretung weiter. Alt-Params
 * werden in das neue Drei-Parameter-Schema (d, view, block/verlauf)
 * übersetzt; der History-Eintrag wird ersetzt, damit "Zurück" nicht in einer
 * Redirect-Schleife endet.
 */

import { redirect, useSearchParams } from "next/navigation";

import {
  berlinTodayISO,
  isValidISODate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { getWeekRange } from "~/lib/timetable-helpers";
import { useTenantAwarePath } from "~/lib/tenant-path";

export type PlanungRedirectTarget =
  "betreuungsplan" | "dienstplan" | "vertretung";

const MONTH_RE = /^\d{4}-\d{2}$/;
const YEAR_RE = /^\d{4}$/;

function targetFromTab(tab: string | null): PlanungRedirectTarget {
  // Mirrors the old /planung parseTab: unknown or missing tab meant the
  // Betreuungsplan tab.
  if (tab === "dienstplan" || tab === "vertretung") return tab;
  return "betreuungsplan";
}

/**
 * Resolves the `d` anchor from the legacy params. Precedence follows the
 * translation table: an explicit `day` wins, then a `week` offset relative
 * to today, then (Betreuungsplan only) the `month`/`year` view anchors.
 */
function resolveAnchorDate(
  params: URLSearchParams,
  today: string,
  target: PlanungRedirectTarget,
): string | null {
  // isValidISODate statt reinem Shape-Check: "2026-02-31" würde sonst als
  // `d` weitergereicht und erst in der Zielansicht still überlaufen.
  const day = params.get("day");
  if (day && isValidISODate(day)) return day;

  const week = params.get("week");
  if (week !== null) {
    const offset = Number.parseInt(week, 10);
    if (Number.isFinite(offset)) {
      // `?week=N` -> `d` = Montag der Zielwoche, invers zu weekOffsetForISO in
      // vertretungsplan-view.tsx (docs/07 §1). Erst auf den Montag der Woche um
      // `today` snappen, dann den Offset addieren — sonst würde N*7 Tage auf
      // einen beliebigen Wochentag addiert und ein nicht-montäglicher Tag
      // herauskommen. getWeekRange ankert intern auf den Montag.
      return toISODate(getWeekRange(parseISODate(today), offset).from);
    }
  }

  if (target === "betreuungsplan") {
    // Legacy month param is self-contained ("YYYY-MM", betreuungsplan-view
    // parseMonth); the year view carried a bare "YYYY".
    const month = params.get("month");
    if (month && MONTH_RE.test(month)) return `${month}-01`;
    const year = params.get("year");
    if (year && YEAR_RE.test(year)) return `${year}-01-01`;
  }

  return null;
}

function translateView(raw: string | null): string | null {
  switch (raw) {
    case "week":
      return "woche";
    case "month":
      return "monat";
    case "series":
    case "templates":
      return "serien";
    case "year":
      // Die Jahresansicht existiert im neuen Vokabular nicht
      // (docs/03 offener Punkt 1, entschieden in docs/06: entfällt).
      return "monat";
    default:
      // "day" und Unbekanntes entfallen; der Tag steckt bereits in `d`.
      return null;
  }
}

/**
 * Pure translation of a legacy planning URL into the new route + params.
 * Exported for table-driven tests; `today` is injected so expectations can
 * pin a fixed date. Returns a href relative to the tenant root.
 */
export function resolvePlanungRedirect(
  params: URLSearchParams,
  today: string,
  target?: PlanungRedirectTarget,
): string {
  const resolvedTarget = target ?? targetFromTab(params.get("tab"));
  const next = new URLSearchParams();

  if (resolvedTarget === "dienstplan") {
    // Der Dienstplan hatte nie URL-Zustand; nichts zu übersetzen.
    return "/dienstplan";
  }

  const anchor = resolveAnchorDate(params, today, resolvedTarget);
  if (anchor) next.set("d", anchor);

  if (resolvedTarget === "betreuungsplan") {
    const view = translateView(params.get("view"));
    if (view) next.set("view", view);
    const instance = params.get("instance");
    if (instance) next.set("block", instance);
    // `history` entfällt: der Verlauf lebt im Vertretungs-Bereich.
    // `period` entfällt ersatzlos: der Zeitraum ist abgeleiteter Zustand.
  } else {
    const instance = params.get("instance");
    if (instance) next.set("block", instance);
    if (params.get("history") === "1") next.set("verlauf", "1");
  }

  const query = next.toString();
  return query ? `/${resolvedTarget}?${query}` : `/${resolvedTarget}`;
}

export function PlanungRedirect({
  target,
}: {
  /** Fixed target for the old stubs; omitted on /planung (derived from ?tab=). */
  target?: PlanungRedirectTarget;
}) {
  const searchParams = useSearchParams();
  const tenantPath = useTenantAwarePath();
  const params = new URLSearchParams(searchParams.toString());
  // Berlin-Kalendertag (docs/07 §1), damit der `d`-Anker mit dem
  // serverseitigen timezone.TodayDate() übereinstimmt.
  return redirect(
    tenantPath(resolvePlanungRedirect(params, berlinTodayISO(), target)),
  );
}

"use client";

// Tagesübersicht der Klassenansicht ("moto schule", #1772/#2294).
//
// Die Startseite beantwortet genau zwei Fragen: welchen Tag sehe ich, und
// welche meiner Klassen muss ich heute überhaupt aufmachen. Die Kinderlisten
// liegen eine Ebene tiefer, eine Klasse pro Seite — vorher schaltete ein
// Klick auf eine Klassenkarte etwas um, das mehrere Bildschirmhöhen weiter
// unten stand und deshalb unsichtbar blieb.
//
// Design follows the Anmeldungen/Planung surface language: one calm content
// section with an uppercase kicker, gray-50 stat blocks, no colored
// dashboards.

import { ChevronLeft, ChevronRight } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useCallback, useMemo } from "react";
import type { CSSProperties } from "react";
import { Button, ButtonLink } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { EmptyState } from "~/components/ui/empty-state";
import NavigationLink from "~/components/ui/navigation-link";
import { Skeleton } from "~/components/ui/skeleton";
import { StatCard } from "~/components/ui/stat-card";
import { getUserDisplayName } from "~/lib/auth-utils";
import { LOCATION_COLORS, MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import { useTimeBasedGreeting } from "~/lib/greeting";
import type { ClassDayReport } from "~/lib/class-day-api";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { schoolClassLabel } from "~/lib/school-class-label";
import type {
  ClassDayClasses,
  SchoolDayChild,
  SchoolDayReport,
} from "~/lib/school-class-day-api";
import { schoolPath } from "~/lib/school-url";
import { useSWRAuth } from "~/lib/swr";
import { countDayChanges } from "./day-changes";
import { classDayDateParam, isWeekendISO } from "./routes";
import {
  schoolDayChildrenByClass,
  schoolDayClassPath,
  schoolDayListParam,
  schoolDayListPath,
  visibleSchoolDayLists,
} from "./school-day";
import type { SchoolDayListKey } from "./school-day";
import { statusLabel } from "./status-labels";
import { useClassDayRefresh } from "./use-class-day-refresh";

const logger = createLogger({ component: "ClassDayOverview" });

// Die Ansicht bleibt bei Übergaben lange offen — sie muss sich selbst
// aktualisieren, damit z. B. eine um 11:30 gemeldete Krankmeldung nicht
// bis zum Reload unsichtbar bleibt. Kein SSE für diese Daten (Lehrkräfte
// betreuen keine aktiven Gruppen), daher Fokus-Revalidierung: SWR feuert
// sie bei window-focus UND visibilitychange, also bei jedem menschlichen
// "Blatt nachsehen". BEWUSST kein refreshInterval — jeder Abruf baut
// serverseitig den vollen ClassRoster pro deckender Phase und Klasse neu,
// und ein Intervall macht daraus Dauerlast aus untätig offenen Tabs.
// Das GDPR-Access-Log flutet dabei nicht: recordClassDayViewAudit
// dedupliziert serverseitig auf eine Zeile pro Actor, Klasse, Datum und
// Zugriffstag.
// Fokus-Revalidierung gedrosselt: schnelles Tab-Hüpfen darf den
// Roster-Rebuild nicht im Sekundentakt auslösen (SWR-Default wären 5s).
const REPORT_FOCUS_THROTTLE_MS = 60 * 1000;

/**
 * Die Zahlen einer Klassenzeile. Kommen aus dem Klassenreport oder, für die
 * Schulverwaltung, aus der Tagesübersicht der ganzen Schule (#4802): beide
 * zählen nach denselben Regeln, die Zeile liest sich gleich.
 */
interface ClassLine {
  school_day: boolean;
  enrollment_known: boolean;
  totals: {
    students: number;
    staying: number;
    leaving: number;
    absent: number;
    list_entries: number;
  };
  changes: number;
}

function reportLine(report: ClassDayReport): ClassLine {
  return {
    school_day: report.school_day,
    enrollment_known: report.enrollment_known,
    totals: report.totals,
    changes: countDayChanges(report),
  };
}

function schoolDayLines(report: SchoolDayReport): Record<string, ClassLine> {
  const lines: Record<string, ClassLine> = {};
  for (const klass of report.classes) {
    lines[klass.school_class] = {
      school_day: report.school_day,
      enrollment_known: report.enrollment_known,
      totals: klass.totals,
      changes: klass.totals.changes,
    };
  }
  return lines;
}

/** Zusammenfassung einer Klasse für den Tag, in einer Zeile. */
function classSummary(report: ClassLine, weekend: boolean): string {
  if (weekend || !report.school_day) return "Kein Schultag";
  const totals = report.totals;
  if (!report.enrollment_known) {
    return `${totals.students} Kinder im Klassenverband`;
  }
  const parts = [
    `${totals.students} Kinder`,
    `${totals.staying} bleiben`,
    `${totals.leaving} gehen heim`,
  ];
  if (totals.absent > 0) parts.push(`${totals.absent} abgemeldet`);
  if (totals.list_entries > 0) {
    parts.push(`${totals.list_entries} keine Betreuung`);
  }
  return parts.join(" · ");
}

function changesLabel(count: number): string {
  return count === 1
    ? "1 Kind anders als sonst"
    : `${count} Kinder anders als sonst`;
}

// Eine Klasse als Zeile: Name, die Zahlen des Tages und — nur wenn es etwas
// gibt — die Anzahl der Abweichungen. Das ist die eine Aussage, die man
// nicht durch Lesen einer Liste bekommt: welche Klasse muss ich aufmachen.
// Als Link, nicht als Knopf mit unsichtbarer Wirkung: hier passiert wirklich
// ein Seitenwechsel.
function ClassLink({
  klass,
  report,
  failed,
  weekend,
  dateISO,
  openList,
}: Readonly<{
  klass: string;
  report: ClassLine | null;
  failed: boolean;
  weekend: boolean;
  dateISO: string;
  openList: SchoolDayListKey | null;
}>) {
  const changes = report?.school_day ? report.changes : 0;
  const summary = failed
    ? "Konnte nicht geladen werden"
    : report
      ? classSummary(report, weekend)
      : "Wird geladen …";
  return (
    <ButtonLink
      href={schoolPath(schoolDayClassPath(klass, dateISO, openList))}
      variant="surface"
      size="card"
      className="justify-between gap-3 text-left"
    >
      {/* flex-1 + min-w-0: ohne beides schrumpft das Textfeld nicht und die
          Karte läuft auf schmalen Bildschirmen über den Rand hinaus. Die
          Zusammenfassung bricht um, statt zu kürzen — eine abgeschnittene
          Zahl ("3 abgemel…") beantwortet keine Frage. */}
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-semibold text-gray-900">
          {schoolClassLabel(klass)}
        </span>
        <span
          className={`mt-1 block text-xs ${failed ? "text-[var(--class-day-danger)]" : "text-gray-500"}`}
        >
          {summary}
        </span>
        {changes > 0 && (
          <span className="mt-1.5 flex items-center gap-1.5 text-xs font-medium text-gray-700">
            <span
              aria-hidden="true"
              className="h-1.5 w-1.5 shrink-0 rounded-full"
              style={{ backgroundColor: LOCATION_COLORS.WARNING }}
            />
            {changesLabel(changes)}
          </span>
        )}
      </span>
      <ChevronRight
        className="h-5 w-5 shrink-0 text-gray-400"
        aria-hidden="true"
      />
    </ButtonLink>
  );
}

/**
 * Was eine Liste über ein Kind nicht schon selbst sagt: in „Kinder“ und
 * „Fehlen“ der Tagesstatus, in „Kinder“ das Kind ohne Betreuung.
 */
function childNote(
  child: SchoolDayChild,
  list: SchoolDayListKey,
): string | null {
  if (list !== "kinder" && list !== "fehlen") return null;
  if (child.status) return statusLabel(child.status);
  if (list === "kinder" && child.list_entry) return "keine Betreuung";
  return null;
}

export interface ClassDayOverviewProps {
  /** Klassenliste und Wochenendplan der angemeldeten Lehrkraft. */
  readonly fetchClasses: () => Promise<ClassDayClasses>;
  /** Tagesreport einer Klasse — portal-eigene Session. */
  readonly fetchClassDay: (
    schoolClass: string,
    date: string,
  ) => Promise<ClassDayReport>;
  /**
   * Tagesübersicht der ganzen Schule (#4802). Darf die Person sie sehen,
   * lädt die Startseite sie einmal statt einen Report je Klasse.
   */
  readonly fetchSchoolDay?: (date: string) => Promise<SchoolDayReport>;
}

export function ClassDayOverview({
  fetchClasses,
  fetchClassDay,
  fetchSchoolDay,
}: ClassDayOverviewProps) {
  const greeting = useTimeBasedGreeting();
  const { data: session } = useSession();
  const router = useRouter();
  const searchParams = useSearchParams();
  // Der Tag steht in der Adresse, nicht nur im Zustand: nur so führt der
  // Zurück-Weg aus einer Klasse auf denselben Tag zurück.
  const dateISO = classDayDateParam(searchParams.get("tag"));
  const openList = schoolDayListParam(searchParams.get("liste"));
  // Die Klassenliste MUSS mitrevalidieren (App-Default ist
  // revalidateOnFocus: false): bliebe sie auf dem Mount-Stand eingefroren,
  // würde eine entzogene Klasse für immer im Reports-Key stehen (jede
  // Fokus-Revalidierung liefe in denselben 403 und der Teilausfall-Banner
  // bliebe bis zum Reload stehen), und eine neu zugewiesene Klasse erschiene
  // nie.
  const {
    data: classList,
    error: classesError,
    isLoading: classesLoading,
    mutate: reloadClasses,
  } = useSWRAuth("class-day-my-classes", fetchClasses, {
    revalidateOnFocus: true,
    focusThrottleInterval: REPORT_FOCUS_THROTTLE_MS,
  });
  const classes = classList?.classes;
  const weekend =
    isWeekendISO(dateISO) && classList?.weekend_follows_friday !== true;
  // Die Schulverwaltung sieht alle Klassen. Ein Report je Klasse hieße N
  // volle Klassenaufbauten pro Abruf; die Tagesübersicht der Schule liefert
  // dieselben Zahlen aus einem Abruf (#4802).
  const schoolMode =
    fetchSchoolDay !== undefined && classList?.can_view_school_day === true;

  // Alle zugewiesenen Klassen für den Tag parallel laden: die Übersicht
  // braucht von jeder Klasse die Zahlen und die Anzahl der Abweichungen.
  // allSettled, damit EINE fehlschlagende Klasse (z. B. 403 nach entzogener
  // Zuweisung) nicht die gesunden Klassen mit wegwischt. Wochenenden laden
  // gar nicht (spart pro Klasse den vollen Report samt GDPR-Logzeile).
  const {
    data: dayData,
    isLoading: reportsLoading,
    mutate: reloadReports,
  } = useSWRAuth(
    classes && classes.length > 0 && !weekend && !schoolMode
      ? `class-day-reports-${dateISO}-${JSON.stringify(classes)}`
      : null,
    async () => {
      const list = classes ?? [];
      const results = await Promise.allSettled(
        list.map((klass) =>
          fetchClassDay(klass, dateISO).then(
            (response) => [klass, response] as const,
          ),
        ),
      );
      const loaded: Record<string, ClassDayReport> = {};
      const failed: string[] = [];
      // Der erste Fehlschlag trägt Code und Vorgangskennung in die Meldung.
      let firstFailure: unknown = null;
      for (const [index, result] of results.entries()) {
        if (result.status === "fulfilled") {
          const [klass, response] = result.value;
          loaded[klass] = response;
        } else {
          const klass = list[index];
          if (klass) failed.push(klass);
          firstFailure ??= result.reason;
          logger.error("class_day_fetch_failed", {
            date: dateISO,
            error:
              result.reason instanceof Error
                ? result.reason.message
                : String(result.reason),
          });
        }
      }
      return { reports: loaded, failed, firstFailure };
    },
    {
      revalidateOnFocus: true,
      focusThrottleInterval: REPORT_FOCUS_THROTTLE_MS,
      // Beim Datumswechsel Skeleton statt der Daten des alten Tages zeigen;
      // bereits besuchte Tage kommen trotzdem sofort aus dem SWR-Cache.
      keepPreviousData: false,
    },
  );

  // Die Tagesübersicht der Schule: dieselbe Fokus-Revalidierung wie die
  // Klassenreports, und der Server protokolliert sie wie diese nur einmal
  // pro Person, Tag und Zugriffstag.
  const {
    data: schoolDay,
    error: schoolDayError,
    isLoading: schoolDayLoading,
    mutate: reloadSchoolDay,
  } = useSWRAuth(
    schoolMode && classes && classes.length > 0 && !weekend
      ? `class-day-school-day-${dateISO}`
      : null,
    () =>
      fetchSchoolDay
        ? fetchSchoolDay(dateISO)
        : Promise.reject(new Error("school day not available")),
    {
      revalidateOnFocus: true,
      focusThrottleInterval: REPORT_FOCUS_THROTTLE_MS,
      keepPreviousData: false,
    },
  );

  // Eine neue Krankmeldung an dieser Schule aktualisiert die Zahlen der
  // Übersicht ohne Neuladen der Seite (#4792); in der Schulansicht die
  // Tagesübersicht der Schule, sonst die Klassenreports.
  const refreshReports = useCallback(
    () => void (schoolMode ? reloadSchoolDay() : reloadReports()),
    [schoolMode, reloadSchoolDay, reloadReports],
  );
  useClassDayRefresh(refreshReports);

  const reports = useMemo<Record<string, ClassLine>>(() => {
    if (schoolMode) return schoolDay ? schoolDayLines(schoolDay) : {};
    const lines: Record<string, ClassLine> = {};
    for (const [klass, report] of Object.entries(dayData?.reports ?? {})) {
      lines[klass] = reportLine(report);
    }
    return lines;
  }, [schoolMode, schoolDay, dayData]);
  const failedClasses = useMemo(
    () => new Set(dayData?.failed ?? []),
    [dayData],
  );
  const loading =
    classesLoading ||
    (!weekend &&
      (classes?.length ?? 0) > 0 &&
      (schoolMode
        ? schoolDay === undefined && schoolDayLoading
        : dayData === undefined && reportsLoading));
  // Fehler beim Laden der Klassenliste ist NICHT "keine Klassen zugewiesen":
  // sonst rennt die Lehrkraft bei einem transienten 500 der Verwaltung
  // hinterher. Katalogtext mit Wiederholen (#2517); bei Teilausfall nennt
  // die Meldung die erste fehlende Klasse, die geladenen bleiben stehen.
  const firstFailedClass = dayData?.failed[0];
  const loadError = useSwrLoadError(
    classesError ??
      (schoolMode ? schoolDayError : dayData?.firstFailure) ??
      null,
    classesError
      ? "die Liste Ihrer Klassen"
      : schoolMode
        ? "die Tagesübersicht der Schule"
        : firstFailedClass
          ? `die Klasse ${schoolClassLabel(firstFailedClass)}`
          : "die Klassenansicht",
    () => {
      if (classesError) return reloadClasses();
      return schoolMode ? reloadSchoolDay() : reloadReports();
    },
  );
  const hasLoadFailure =
    Boolean(classesError) ||
    (schoolMode ? Boolean(schoolDayError) : (dayData?.failed.length ?? 0) > 0);

  const selectedDate = parseISODate(dateISO);
  // Eine geöffnete Liste bleibt beim Tageswechsel offen: dieselbe Frage
  // für den nächsten Tag.
  const goToDate = (next: string) => {
    router.replace(schoolPath(schoolDayListPath(next, openList)), {
      scroll: false,
    });
  };
  const shiftDay = (delta: number) => {
    const next = new Date(selectedDate);
    next.setDate(next.getDate() + delta);
    goToDate(toISODate(next));
  };

  const noClasses = classes !== undefined && classes.length === 0;

  // Die Zahlen der ganzen Schule für die Schulverwaltung (#4802). Jede Zahl
  // ist ein Link auf dieselbe Seite mit `?liste=…`: darunter stehen dann die
  // Kinder dieser Zahl, nach Klasse gruppiert. Ein echter Seitenwechsel im
  // selben Fenster, keine versteckte Schaltfläche, und der Zurück-Weg aus
  // einer Klasse landet wieder bei der Liste.
  const schoolLists = schoolDay ? visibleSchoolDayLists(schoolDay) : [];
  const open = schoolLists.find((list) => list.key === openList) ?? null;
  const groups =
    schoolDay && open ? schoolDayChildrenByClass(schoolDay, open) : [];

  return (
    <div
      className="w-full"
      style={
        {
          "--class-day-blue": MOTO_COLOR_PALETTE.blue.base,
          "--class-day-danger": MOTO_COLOR_PALETTE.red.base,
        } as CSSProperties
      }
    >
      <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p className="text-xs font-semibold tracking-wide text-[var(--class-day-blue)] uppercase">
              Klassenansicht
            </p>
            <h2 className="mt-1 text-base font-semibold text-gray-900">
              {greeting}, {getUserDisplayName(session)}
            </h2>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
              Ihre Übergabe nach Unterricht am {formatDate(dateISO)}. Öffnen Sie
              eine Klasse, um zu sehen, wer in Randstunde oder Ganztag bleibt
              und wer nach Hause geht.
            </p>
          </div>
          <div className="flex w-full items-center gap-1 lg:w-auto">
            <Button
              type="button"
              variant="outline"
              size="icon"
              aria-label="Vorheriger Tag"
              onClick={() => shiftDay(-1)}
              className="h-9 w-9 shrink-0 rounded-lg bg-white"
            >
              <ChevronLeft className="h-4 w-4" aria-hidden="true" />
            </Button>
            <DatePicker
              value={selectedDate}
              onChange={(date) => {
                if (date) goToDate(toISODate(date));
              }}
              calendarLayout="popover"
              hideClearButton
              className="min-w-0 flex-1 sm:w-44 sm:flex-none"
            />
            <Button
              type="button"
              variant="outline"
              size="icon"
              aria-label="Nächster Tag"
              onClick={() => shiftDay(1)}
              className="h-9 w-9 shrink-0 rounded-lg bg-white"
            >
              <ChevronRight className="h-4 w-4" aria-hidden="true" />
            </Button>
          </div>
        </div>

        {noClasses && (
          <EmptyState
            className="mt-4"
            title="Keine Klassen zugewiesen"
            description="Ihrem Konto ist noch keine Klasse zugeordnet. Die OGS-Verwaltung kann Ihnen Klassen unter Mitarbeitende zuweisen."
          />
        )}

        {!noClasses && loading && (
          <div className="mt-4 grid gap-3 lg:grid-cols-2">
            <Skeleton className="h-24 w-full" />
            <Skeleton className="h-24 w-full" />
          </div>
        )}

        {/* Voller Fehlerzustand nur, wenn GAR NICHTS geladen werden konnte;
            bei Teilausfall bleiben die gesunden Klassen stehen. */}
        {!noClasses &&
          !loading &&
          hasLoadFailure &&
          Object.keys(reports).length === 0 &&
          !weekend && <LoadErrorAlert error={loadError} className="mt-4" />}

        {!noClasses &&
          !loading &&
          (weekend || Object.keys(reports).length > 0) && (
            <>
              {hasLoadFailure && Object.keys(reports).length > 0 && (
                <LoadErrorAlert error={loadError} className="mt-4" />
              )}

              {weekend ? (
                <EmptyState
                  className="mt-4"
                  title="Kein Schultag"
                  description="Für Samstag und Sonntag gibt es keine Übergabe. Bitte einen Wochentag wählen."
                />
              ) : (
                <>
                  {schoolMode && schoolDay && (
                    <>
                      <h3 className="mt-5 text-xs font-semibold tracking-wide text-gray-500 uppercase">
                        Ganze Schule
                      </h3>
                      <div className="mt-2 grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
                        {schoolLists.map((list) => (
                          <StatCard
                            key={list.key}
                            variant="row"
                            label={list.label}
                            value={list.count(schoolDay.totals)}
                            href={schoolPath(
                              schoolDayListPath(dateISO, list.key),
                            )}
                          />
                        ))}
                      </div>
                      {open && (
                        <div className="mt-4">
                          <div className="flex items-center justify-between gap-3">
                            <h3 className="text-sm font-semibold text-gray-900">
                              {open.label}: {open.count(schoolDay.totals)}
                            </h3>
                            <ButtonLink
                              href={schoolPath(
                                schoolDayListPath(dateISO, null),
                              )}
                              variant="ghost"
                              size="compact"
                            >
                              Liste schließen
                            </ButtonLink>
                          </div>
                          {groups.length === 0 ? (
                            <p className="mt-2 text-sm text-gray-500">
                              An diesem Tag stehen keine Kinder in dieser Liste.
                            </p>
                          ) : (
                            <div className="mt-2 grid gap-3 lg:grid-cols-2">
                              {groups.map((group) => (
                                <div
                                  key={group.schoolClass}
                                  className="rounded-xl bg-gray-50 px-3 py-2"
                                >
                                  <NavigationLink
                                    href={schoolPath(
                                      schoolDayClassPath(
                                        group.schoolClass,
                                        dateISO,
                                        open.key,
                                      ),
                                    )}
                                    className="text-sm font-semibold text-gray-900 underline-offset-2 hover:underline"
                                  >
                                    {schoolClassLabel(group.schoolClass)} ·{" "}
                                    {group.children.length}
                                  </NavigationLink>
                                  <ul className="mt-1 space-y-0.5 text-sm text-gray-700">
                                    {group.children.map((child) => {
                                      const note = childNote(child, open.key);
                                      return (
                                        <li
                                          key={
                                            child.list_entry
                                              ? `entry-${child.list_entry_id}`
                                              : `student-${child.student_id}`
                                          }
                                        >
                                          {child.last_name}, {child.first_name}
                                          {note && (
                                            <span className="text-gray-500">
                                              {" "}
                                              · {note}
                                            </span>
                                          )}
                                        </li>
                                      );
                                    })}
                                  </ul>
                                </div>
                              ))}
                            </div>
                          )}
                        </div>
                      )}
                    </>
                  )}
                  <h3 className="mt-5 text-xs font-semibold tracking-wide text-gray-500 uppercase">
                    {schoolMode ? "Alle Klassen" : "Ihre Klassen"}
                  </h3>
                  <div className="mt-2 grid gap-3 lg:grid-cols-2">
                    {(classes ?? []).map((klass) => (
                      <ClassLink
                        key={klass}
                        klass={klass}
                        report={reports[klass] ?? null}
                        failed={failedClasses.has(klass)}
                        weekend={weekend}
                        dateISO={dateISO}
                        openList={schoolMode ? openList : null}
                      />
                    ))}
                  </div>
                </>
              )}
            </>
          )}
      </section>
    </div>
  );
}

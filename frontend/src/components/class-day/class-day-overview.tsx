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
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useMemo } from "react";
import type { CSSProperties } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { Skeleton } from "~/components/ui/skeleton";
import { getUserDisplayName } from "~/lib/auth-utils";
import { LOCATION_COLORS, MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import { getTimeBasedGreeting } from "~/lib/greeting";
import type { ClassDayReport } from "~/lib/class-day-api";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { schoolClassLabel } from "~/lib/school-class-label";
import { schoolPath } from "~/lib/school-url";
import { useSWRAuth } from "~/lib/swr";
import { countDayChanges } from "./day-changes";
import { classDayDateParam, classDayPath, isWeekendISO } from "./routes";

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

/** Zusammenfassung einer Klasse für den Tag, in einer Zeile. */
function classSummary(report: ClassDayReport, weekend: boolean): string {
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
}: Readonly<{
  klass: string;
  report: ClassDayReport | null;
  failed: boolean;
  weekend: boolean;
  dateISO: string;
}>) {
  const changes = countDayChanges(report);
  const summary = failed
    ? "Konnte nicht geladen werden"
    : report
      ? classSummary(report, weekend)
      : "Wird geladen …";
  return (
    <Link
      href={schoolPath(classDayPath(klass, dateISO))}
      className="flex items-center justify-between gap-3 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
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
    </Link>
  );
}

export interface ClassDayOverviewProps {
  /** Klassenliste der angemeldeten Lehrkraft — portal-eigene Session. */
  readonly fetchMyClasses: () => Promise<string[]>;
  /** Tagesreport einer Klasse — portal-eigene Session. */
  readonly fetchClassDay: (
    schoolClass: string,
    date: string,
  ) => Promise<ClassDayReport>;
}

export function ClassDayOverview({
  fetchMyClasses,
  fetchClassDay,
}: ClassDayOverviewProps) {
  const { data: session } = useSession();
  const router = useRouter();
  const searchParams = useSearchParams();
  // Der Tag steht in der Adresse, nicht nur im Zustand: nur so führt der
  // Zurück-Weg aus einer Klasse auf denselben Tag zurück.
  const dateISO = classDayDateParam(searchParams.get("tag"));
  const weekend = isWeekendISO(dateISO);

  // Die Klassenliste MUSS mitrevalidieren (App-Default ist
  // revalidateOnFocus: false): bliebe sie auf dem Mount-Stand eingefroren,
  // würde eine entzogene Klasse für immer im Reports-Key stehen (jede
  // Fokus-Revalidierung liefe in denselben 403 und der Teilausfall-Banner
  // bliebe bis zum Reload stehen), und eine neu zugewiesene Klasse erschiene
  // nie.
  const {
    data: classes,
    error: classesError,
    isLoading: classesLoading,
  } = useSWRAuth("class-day-my-classes", fetchMyClasses, {
    revalidateOnFocus: true,
    focusThrottleInterval: REPORT_FOCUS_THROTTLE_MS,
  });

  // Alle zugewiesenen Klassen für den Tag parallel laden: die Übersicht
  // braucht von jeder Klasse die Zahlen und die Anzahl der Abweichungen.
  // allSettled, damit EINE fehlschlagende Klasse (z. B. 403 nach entzogener
  // Zuweisung) nicht die gesunden Klassen mit wegwischt. Wochenenden laden
  // gar nicht (spart pro Klasse den vollen Report samt GDPR-Logzeile).
  const { data: dayData, isLoading: reportsLoading } = useSWRAuth(
    classes && classes.length > 0 && !weekend
      ? `class-day-reports-${dateISO}-${classes.join("|")}`
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
      for (const [index, result] of results.entries()) {
        if (result.status === "fulfilled") {
          const [klass, response] = result.value;
          loaded[klass] = response;
        } else {
          const klass = list[index];
          if (klass) failed.push(klass);
          logger.error("class_day_fetch_failed", {
            date: dateISO,
            error:
              result.reason instanceof Error
                ? result.reason.message
                : String(result.reason),
          });
        }
      }
      return { reports: loaded, failed };
    },
    {
      revalidateOnFocus: true,
      focusThrottleInterval: REPORT_FOCUS_THROTTLE_MS,
      // Beim Datumswechsel Skeleton statt der Daten des alten Tages zeigen;
      // bereits besuchte Tage kommen trotzdem sofort aus dem SWR-Cache.
      keepPreviousData: false,
    },
  );

  const reports = useMemo(() => dayData?.reports ?? {}, [dayData]);
  const failedClasses = useMemo(
    () => new Set(dayData?.failed ?? []),
    [dayData],
  );
  const loading =
    classesLoading ||
    (!weekend &&
      (classes?.length ?? 0) > 0 &&
      dayData === undefined &&
      reportsLoading);
  // Fehler beim Laden der Klassenliste ist NICHT "keine Klassen zugewiesen":
  // sonst rennt die Lehrkraft bei einem transienten 500 der Verwaltung
  // hinterher.
  const error = classesError
    ? "Die Klassenansicht konnte nicht geladen werden."
    : failedClasses.size > 0
      ? Object.keys(reports).length === 0
        ? "Die Klassenansicht konnte nicht geladen werden."
        : "Nicht alle Klassen konnten geladen werden. Bitte laden Sie die Seite neu."
      : null;

  const selectedDate = parseISODate(dateISO);
  const goToDate = (next: string) => {
    router.replace(`${schoolPath("/school")}?tag=${next}`, { scroll: false });
  };
  const shiftDay = (delta: number) => {
    const next = new Date(selectedDate);
    next.setDate(next.getDate() + delta);
    goToDate(toISODate(next));
  };

  const noClasses = classes !== undefined && classes.length === 0;

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
              {getTimeBasedGreeting()}, {getUserDisplayName(session)}
            </h2>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
              Ihre Übergabe nach Unterricht am {formatDate(dateISO)}. Öffnen Sie
              eine Klasse, um zu sehen, wer in Randstunde oder Ganztag bleibt
              und wer nach Hause geht.
            </p>
          </div>
          <div className="flex items-center gap-1">
            <Button
              type="button"
              variant="outline"
              size="icon"
              aria-label="Vorheriger Tag"
              onClick={() => shiftDay(-1)}
              className="h-9 w-9 rounded-lg bg-white"
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
              className="w-full sm:w-44"
            />
            <Button
              type="button"
              variant="outline"
              size="icon"
              aria-label="Nächster Tag"
              onClick={() => shiftDay(1)}
              className="h-9 w-9 rounded-lg bg-white"
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
          error !== null &&
          Object.keys(reports).length === 0 &&
          !weekend && (
            <EmptyState
              className="mt-4"
              title="Klassenansicht nicht verfügbar"
              description={error}
            />
          )}

        {!noClasses &&
          !loading &&
          (weekend || Object.keys(reports).length > 0) && (
            <>
              {error !== null && Object.keys(reports).length > 0 && (
                <div className="mt-4">
                  <Alert type="error" message={error} />
                </div>
              )}

              {weekend ? (
                <EmptyState
                  className="mt-4"
                  title="Kein Schultag"
                  description="Für Samstag und Sonntag gibt es keine Übergabe. Bitte einen Wochentag wählen."
                />
              ) : (
                <>
                  <h3 className="mt-5 text-xs font-semibold tracking-wide text-gray-500 uppercase">
                    Ihre Klassen
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

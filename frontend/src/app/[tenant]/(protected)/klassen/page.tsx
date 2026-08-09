"use client";

// Lehrkraft-Klassenansicht (#1772): read-only Tagesansicht pro zugewiesener
// Klasse für die Übergabe nach Unterricht — wer bleibt heute in der
// Betreuung, wer geht nach Hause (und wie), wer ist abgemeldet. Datenbasis
// ist der Klassenlisten-Report, serverseitig auf die zugewiesenen Klassen
// gescopt und ohne Kontaktdaten der Sorgeberechtigten.
//
// Design follows the Anmeldungen/Planung surface language: one calm content
// section with an uppercase kicker, gray-50 stat blocks, status sections
// like the Tagesauswertung (day-log) instead of colored dashboards.

import { ChevronLeft, ChevronRight } from "lucide-react";
import { useSession } from "next-auth/react";
import { useMemo, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { getUserDisplayName } from "~/lib/auth-utils";
import { getTimeBasedGreeting } from "~/lib/greeting";
import {
  fetchClassDay,
  fetchMyClasses,
  type ClassDayReport,
  type ClassDayRow,
} from "~/lib/class-day-api";
import {
  berlinTodayISO,
  formatDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";

const logger = createLogger({ component: "KlassenPage" });

const STATUS_LABELS: Record<string, string> = {
  sick: "Krank",
  excused: "Entschuldigt",
  class_trip: "Klassenfahrt",
  // Abhol-Ausnahme ohne Zeit: die Betreuung für diesen Tag wurde abgesagt.
  cancelled: "Heute abgemeldet",
};

// Muted status colors from the brand table (LOCATION_COLORS semantics):
// SICK amber, CLASS_TRIP blue, EXCUSED purple, CANCELLED neutral gray.
const STATUS_COLORS: Record<string, string> = {
  sick: "#EAB308",
  class_trip: "#5080D8",
  excused: "#7C3AED",
  cancelled: "#6B7280",
};

// Klassennamen sind Freitext — manche Schulen speichern "1a", andere schon
// "Klasse 1a". Kein doppeltes Präfix anzeigen.
function classLabel(klass: string): string {
  return /^klasse\b/i.test(klass.trim()) ? klass.trim() : `Klasse ${klass}`;
}

function Stat({ label, value }: Readonly<{ label: string; value: number }>) {
  return (
    <div className="rounded-xl bg-gray-50 px-3 py-2">
      <span className="block text-sm font-semibold text-gray-900">{value}</span>
      <span className="block text-[11px] font-medium text-gray-500">
        {label}
      </span>
    </div>
  );
}

function rowDetailLine(row: ClassDayRow, enrollmentKnown: boolean): string {
  const parts: string[] = [];
  if (row.stays_today && row.offerings.length > 0) {
    parts.push(row.offerings.join(", "));
  }
  if (row.departure) parts.push(row.departure);
  // Ohne abdeckende Anmeldephase ist "keine OGS-Anmeldung" keine Aussage,
  // sondern genau das, was enrollment_known als unbekannt deklariert.
  if (enrollmentKnown && !row.registered) parts.push("keine OGS-Anmeldung");
  return parts.join(" · ");
}

function StudentRow({
  row,
  enrollmentKnown,
}: {
  readonly row: ClassDayRow;
  readonly enrollmentKnown: boolean;
}) {
  const detail = rowDetailLine(row, enrollmentKnown);
  return (
    <li className="flex items-center justify-between gap-3 rounded-xl border border-gray-100 bg-white px-3 py-2.5">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-gray-900">
          {row.last_name}, {row.first_name}
        </p>
        {detail ? (
          <p className="truncate text-xs text-gray-500">{detail}</p>
        ) : null}
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {/* "bis HH:MM" nur für Kinder, die bleiben — bei Heimgehern und
            Abgemeldeten wäre die Abholzeit irreführend. */}
        {row.stays_today && row.pickup ? (
          <span className="text-xs font-medium text-gray-600 tabular-nums">
            bis {row.pickup}
          </span>
        ) : null}
        {row.status ? (
          <StatusDotBadge
            label={STATUS_LABELS[row.status] ?? row.status}
            color={STATUS_COLORS[row.status] ?? "#6B7280"}
          />
        ) : null}
      </div>
    </li>
  );
}

function Section({
  title,
  count,
  accent = "text-gray-500",
  rows,
  enrollmentKnown = true,
}: Readonly<{
  title: string;
  count: number;
  accent?: string;
  rows: ClassDayRow[];
  enrollmentKnown?: boolean;
}>) {
  if (rows.length === 0) return null;
  return (
    <div>
      <h3
        className={`mb-2 text-xs font-semibold tracking-wide uppercase ${accent}`}
      >
        {title} ({count})
      </h3>
      {/* Zweispaltig ab lg, damit eine volle Klasse nicht zu einer langen
          schmalen Liste wird. */}
      <ul className="grid gap-1.5 lg:grid-cols-2">
        {rows.map((row) => (
          <StudentRow
            key={row.student_id}
            row={row}
            enrollmentKnown={enrollmentKnown}
          />
        ))}
      </ul>
    </div>
  );
}

// ClassCard fasst eine Klasse für den gewählten Tag zusammen und ist
// zugleich der Umschalter für die Detail-Listen darunter (Muster: die
// GroupCards der Tagesauswertung, nur klickbar).
function ClassCard({
  klass,
  report,
  failed,
  weekend,
  selected,
  onSelect,
}: Readonly<{
  klass: string;
  report: ClassDayReport | null;
  failed: boolean;
  weekend: boolean;
  selected: boolean;
  onSelect: () => void;
}>) {
  const totals = report?.totals;
  const schoolDay = report?.school_day ?? true;
  // Bleiben/Gehen ist nur mit abdeckender Anmeldephase eine ehrliche Zahl.
  const splitKnown = schoolDay && (report?.enrollment_known ?? false);
  const subtitle = failed
    ? "Konnte nicht geladen werden"
    : weekend
      ? "Kein Schultag"
      : totals
        ? splitKnown
          ? `${totals.staying} von ${totals.students} Kindern bleiben in der Betreuung`
          : `${totals.students} Kinder im Klassenverband`
        : "Wird geladen …";
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      className={`rounded-2xl border bg-white p-4 text-left shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
        selected
          ? "border-[#5080D8] ring-1 ring-[#5080D8]/30"
          : "border-gray-200 hover:border-gray-300"
      }`}
    >
      <h3 className="truncate text-sm font-semibold text-gray-900">
        {classLabel(klass)}
      </h3>
      <p
        className={`mt-1 text-xs ${failed ? "text-[#CC2626]" : "text-gray-500"}`}
      >
        {subtitle}
      </p>
      {totals && !failed && !weekend && (
        <div
          className={`mt-3 grid gap-2 ${splitKnown ? "grid-cols-2 sm:grid-cols-4" : "grid-cols-1"}`}
        >
          <Stat label="Klassenverband" value={totals.students} />
          {splitKnown && (
            <>
              <Stat label="Bleiben" value={totals.staying} />
              <Stat label="Gehen heim" value={totals.leaving} />
              <Stat label="Abgemeldet" value={totals.absent} />
            </>
          )}
        </div>
      )}
    </button>
  );
}

// Samstag/Sonntag haben keine Übergabe — client-seitig erkennbar, also gar
// nicht erst laden (spart pro Klasse den vollen Report samt GDPR-Logzeile).
function isWeekendISO(dateISO: string): boolean {
  const day = parseISODate(dateISO).getDay();
  return day === 0 || day === 6;
}

// Die Ansicht bleibt bei Übergaben lange offen — sie muss sich selbst
// aktualisieren, damit z. B. eine um 11:30 gemeldete Krankmeldung nicht
// bis zum Reload unsichtbar bleibt. Kein SSE für diese Daten (Lehrkräfte
// betreuen keine aktiven Gruppen), daher Intervall plus Fokus-Revalidierung.
// Das GDPR-Access-Log flutet dabei nicht: recordClassDayViewAudit dedupliziert
// serverseitig auf eine Zeile pro Actor, Klasse, Datum und Zugriffstag.
const REPORT_REFRESH_INTERVAL_MS = 5 * 60 * 1000;
// Fokus-Revalidierung gedrosselt: jeder Abruf baut serverseitig den vollen
// ClassRoster pro deckender Phase und Klasse neu — schnelles Tab-Hüpfen darf
// das nicht im Sekundentakt auslösen (SWR-Default wären 5 Sekunden).
const REPORT_FOCUS_THROTTLE_MS = 60 * 1000;

export default function KlassenPage() {
  const { data: session } = useSession();
  const [selectedClassState, setSelectedClass] = useState<string>("");
  const [dateISO, setDateISO] = useState<string>(() => berlinTodayISO());
  const weekend = isWeekendISO(dateISO);

  const {
    data: classes,
    error: classesError,
    isLoading: classesLoading,
  } = useSWRAuth("class-day-my-classes", fetchMyClasses);

  const selectedClass = selectedClassState || (classes?.[0] ?? "");

  // Alle zugewiesenen Klassen für den Tag parallel laden: die Karten oben
  // zeigen jede Klasse, die Listen darunter die ausgewählte. allSettled,
  // damit EINE fehlschlagende Klasse (z. B. 403 nach entzogener Zuweisung)
  // nicht die gesunden Klassen mit wegwischt. Wochenenden laden gar nicht
  // (spart pro Klasse den vollen Report samt GDPR-Logzeile). SWR liefert
  // stale-while-revalidate beim Zurückblättern und hält die offene Ansicht
  // per Intervall und Fokus-Revalidierung aktuell.
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
      refreshInterval: REPORT_REFRESH_INTERVAL_MS,
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

  const report = selectedClass ? (reports[selectedClass] ?? null) : null;

  const selectedDate = parseISODate(dateISO);
  const shiftDay = (delta: number) => {
    const next = new Date(selectedDate);
    next.setDate(next.getDate() + delta);
    setDateISO(toISODate(next));
  };

  const staying = useMemo(
    () => report?.rows.filter((row) => row.stays_today) ?? [],
    [report],
  );
  const leaving = useMemo(
    () => report?.rows.filter((row) => !row.stays_today && !row.status) ?? [],
    [report],
  );
  const absent = useMemo(
    () => report?.rows.filter((row) => Boolean(row.status)) ?? [],
    [report],
  );
  // Ohne Anmeldedaten (keine abdeckende Phase) gibt es keinen
  // Bleiben/Gehen-Split — alle nicht abgemeldeten Kinder neutral listen.
  const unknownRows = useMemo(
    () => report?.rows.filter((row) => !row.status) ?? [],
    [report],
  );

  const noClasses = classes !== undefined && classes.length === 0;

  return (
    <div className="w-full">
      <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
              Klassenansicht
            </p>
            <h2 className="mt-1 text-base font-semibold text-gray-900">
              {getTimeBasedGreeting()}, {getUserDisplayName(session)}
            </h2>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
              Ihre Übergabe nach Unterricht am {formatDate(dateISO)}: wer bleibt
              in Randstunde oder Ganztag, wer geht nach Hause.
            </p>
          </div>
          <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
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
                  if (date) setDateISO(toISODate(date));
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
        </div>

        {noClasses && (
          <EmptyState
            className="mt-4"
            title="Keine Klassen zugewiesen"
            description="Ihrem Konto ist noch keine Klasse zugeordnet. Die OGS-Verwaltung kann Ihnen Klassen unter Mitarbeitende zuweisen."
          />
        )}

        {!noClasses && loading && (
          <div className="mt-4 space-y-3">
            <div className="grid gap-3 lg:grid-cols-2">
              <Skeleton className="h-32 w-full" />
              <Skeleton className="h-32 w-full" />
            </div>
            <Skeleton className="h-40 w-full" />
          </div>
        )}

        {/* Voller Fehlerzustand nur, wenn GAR NICHTS geladen werden konnte;
            bei Teilausfall bleiben Karten und gesunde Klassen stehen. */}
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

        {/* Karten IMMER rendern, unabhängig vom Zustand der ausgewählten
            Klasse — sonst reißt eine fehlgeschlagene Klasse die gesunden
            mit aus der Ansicht. */}
        {!noClasses &&
          !loading &&
          (weekend || Object.keys(reports).length > 0) && (
            <>
              {error !== null && Object.keys(reports).length > 0 && (
                <div className="mt-4">
                  <Alert type="error" message={error} />
                </div>
              )}

              <div className="mt-4 grid gap-3 lg:grid-cols-2">
                {(classes ?? []).map((klass) => (
                  <ClassCard
                    key={klass}
                    klass={klass}
                    report={reports[klass] ?? null}
                    failed={failedClasses.has(klass)}
                    weekend={weekend}
                    selected={klass === selectedClass}
                    onSelect={() => setSelectedClass(klass)}
                  />
                ))}
              </div>

              {weekend && (
                <EmptyState
                  className="mt-4"
                  title="Kein Schultag"
                  description="Für Samstag und Sonntag gibt es keine Übergabe. Bitte einen Wochentag wählen."
                />
              )}

              {!weekend && !report && failedClasses.has(selectedClass) && (
                <EmptyState
                  className="mt-4"
                  title={`${classLabel(selectedClass)} nicht verfügbar`}
                  description="Diese Klasse konnte nicht geladen werden. Bitte laden Sie die Seite neu oder wählen Sie eine andere Klasse."
                />
              )}

              {!weekend && report && report.rows.length === 0 && (
                <EmptyState
                  className="mt-4"
                  title="Keine Kinder gefunden"
                  description={`Für die Klasse ${report.school_class} sind keine Kinder hinterlegt.`}
                />
              )}

              {/* Ohne abdeckende Anmeldephase ist Bleiben/Gehen unbekannt —
                  neutraler Klassenverband statt "alle gehen nach Hause". */}
              {!weekend &&
                report &&
                report.rows.length > 0 &&
                !report.enrollment_known && (
                  <div className="mt-5 space-y-4 border-t border-gray-100 pt-4">
                    <Alert
                      type="info"
                      message="Für diesen Tag liegt keine Anmeldephase vor. Wer in der Betreuung bleibt, kann deshalb nicht angezeigt werden."
                    />
                    <Section
                      title="Klassenverband"
                      count={unknownRows.length}
                      rows={unknownRows}
                      enrollmentKnown={false}
                    />
                    <Section
                      title="Abgemeldet"
                      count={absent.length}
                      accent="text-[#CC2626]"
                      rows={absent}
                      enrollmentKnown={false}
                    />
                  </div>
                )}

              {!weekend &&
                report &&
                report.rows.length > 0 &&
                report.enrollment_known && (
                  <div className="mt-5 space-y-4 border-t border-gray-100 pt-4">
                    {(classes?.length ?? 0) > 1 && (
                      <h3 className="text-sm font-semibold text-gray-900">
                        {classLabel(report.school_class)} im Detail
                      </h3>
                    )}
                    <Section
                      title="Bleiben in der Betreuung"
                      count={staying.length}
                      accent="text-[#5080D8]"
                      rows={staying}
                    />
                    <Section
                      title="Gehen nach Hause"
                      count={leaving.length}
                      rows={leaving}
                    />
                    <Section
                      title="Abgemeldet"
                      count={absent.length}
                      accent="text-[#CC2626]"
                      rows={absent}
                    />
                  </div>
                )}
            </>
          )}
      </section>
    </div>
  );
}

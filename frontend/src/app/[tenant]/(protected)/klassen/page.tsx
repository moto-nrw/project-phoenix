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
import { useEffect, useMemo, useState } from "react";
import { Button } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { getUserDisplayName } from "~/lib/auth-utils";
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

// Tageszeit-Gruß nach Berliner Uhr — /klassen ist die Startseite der
// Lehrkraft, und ein Einstieg mit Namen wirkt weniger wie eine Unterseite.
function berlinGreeting(now: Date = new Date()): string {
  const hour = Number(
    now.toLocaleString("de-DE", {
      hour: "2-digit",
      hour12: false,
      timeZone: "Europe/Berlin",
    }),
  );
  if (hour < 11) return "Guten Morgen";
  if (hour < 18) return "Guten Tag";
  return "Guten Abend";
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

function rowDetailLine(row: ClassDayRow): string {
  const parts: string[] = [];
  if (row.stays_today && row.offerings.length > 0) {
    parts.push(row.offerings.join(", "));
  }
  if (row.departure) parts.push(row.departure);
  if (!row.registered) parts.push("keine OGS-Anmeldung");
  return parts.join(" · ");
}

function StudentRow({ row }: { readonly row: ClassDayRow }) {
  const detail = rowDetailLine(row);
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
}: Readonly<{
  title: string;
  count: number;
  accent?: string;
  rows: ClassDayRow[];
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
          <StudentRow key={row.student_id} row={row} />
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
  selected,
  onSelect,
}: Readonly<{
  klass: string;
  report: ClassDayReport | null;
  selected: boolean;
  onSelect: () => void;
}>) {
  const totals = report?.totals;
  const schoolDay = report?.school_day ?? true;
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      className={`rounded-2xl border bg-white p-4 text-left shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
        selected
          ? "border-gray-900 ring-1 ring-gray-900"
          : "border-gray-200 hover:border-gray-300"
      }`}
    >
      <h3 className="truncate text-sm font-semibold text-gray-900">
        {classLabel(klass)}
      </h3>
      <p className="mt-1 text-xs text-gray-500">
        {totals
          ? schoolDay
            ? `${totals.staying} von ${totals.students} Kindern bleiben in der Betreuung`
            : `${totals.students} Kinder im Klassenverband`
          : "Wird geladen …"}
      </p>
      {totals && (
        <div
          className={`mt-3 grid gap-2 ${schoolDay ? "grid-cols-2 sm:grid-cols-4" : "grid-cols-1"}`}
        >
          <Stat label="Klassenverband" value={totals.students} />
          {schoolDay && (
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

export default function KlassenPage() {
  const { data: session } = useSession();
  const [classes, setClasses] = useState<string[] | null>(null);
  const [selectedClass, setSelectedClass] = useState<string>("");
  const [dateISO, setDateISO] = useState<string>(() => berlinTodayISO());
  const [reports, setReports] = useState<Record<string, ClassDayReport>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetchMyClasses()
      .then((list) => {
        if (cancelled) return;
        setClasses(list);
        setSelectedClass((current) => current || (list[0] ?? ""));
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setClasses([]);
        logger.error("class_day_classes_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Alle zugewiesenen Klassen für den Tag parallel laden: die Karten oben
  // zeigen jede Klasse, die Listen darunter die ausgewählte.
  useEffect(() => {
    if (!classes || classes.length === 0) {
      if (classes !== null) setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    Promise.all(
      classes.map((klass) =>
        fetchClassDay(klass, dateISO).then(
          (response) => [klass, response] as const,
        ),
      ),
    )
      .then((entries) => {
        if (!cancelled) setReports(Object.fromEntries(entries));
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setReports({});
        setError("Die Klassenansicht konnte nicht geladen werden.");
        logger.error("class_day_fetch_failed", {
          date: dateISO,
          error: err instanceof Error ? err.message : String(err),
        });
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [classes, dateISO]);

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

  const noClasses = classes !== null && classes.length === 0;

  return (
    <div className="w-full">
      <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
              Klassenansicht
            </p>
            <h2 className="mt-1 text-base font-semibold text-gray-900">
              {berlinGreeting()}, {getUserDisplayName(session)}
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

        {!noClasses && !loading && error !== null && (
          <EmptyState
            className="mt-4"
            title="Klassenansicht nicht verfügbar"
            description={error}
          />
        )}

        {!noClasses && !loading && error === null && report && (
          <>
            {/* Eine Karte pro Klasse mit den Tageszahlen — zugleich der
                Umschalter für die Detail-Listen darunter. */}
            <div className="mt-4 grid gap-3 lg:grid-cols-2">
              {(classes ?? []).map((klass) => (
                <ClassCard
                  key={klass}
                  klass={klass}
                  report={reports[klass] ?? null}
                  selected={klass === selectedClass}
                  onSelect={() => setSelectedClass(klass)}
                />
              ))}
            </div>

            {!report.school_day && (
              <EmptyState
                className="mt-4"
                title="Kein Schultag"
                description="Für Samstag und Sonntag gibt es keine Übergabe. Bitte einen Wochentag wählen."
              />
            )}

            {report.school_day && report.rows.length === 0 && (
              <EmptyState
                className="mt-4"
                title="Keine Kinder gefunden"
                description={`Für die Klasse ${report.school_class} sind keine Kinder hinterlegt.`}
              />
            )}

            {report.school_day && report.rows.length > 0 && (
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

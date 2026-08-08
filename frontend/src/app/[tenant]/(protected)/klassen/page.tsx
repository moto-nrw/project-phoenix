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
import { useEffect, useMemo, useState } from "react";
import { DatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
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
};

// Muted status colors from the brand table (LOCATION_COLORS semantics):
// SICK amber, CLASS_TRIP blue, EXCUSED purple.
const STATUS_COLORS: Record<string, string> = {
  sick: "#EAB308",
  class_trip: "#5080D8",
  excused: "#7C3AED",
};

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
  if (row.pickup) parts.push(`bis ${row.pickup} Uhr`);
  if (row.departure) parts.push(row.departure);
  if (!row.registered) parts.push("keine OGS-Anmeldung");
  return parts.join(" · ");
}

function StudentRow({ row }: { readonly row: ClassDayRow }) {
  const detail = rowDetailLine(row);
  return (
    <li className="flex items-center justify-between gap-3 rounded-lg px-3 py-2">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-gray-900">
          {row.last_name}, {row.first_name}
        </p>
        {detail ? (
          <p className="truncate text-xs text-gray-500">{detail}</p>
        ) : null}
      </div>
      {row.status ? (
        <StatusDotBadge
          label={STATUS_LABELS[row.status] ?? row.status}
          color={STATUS_COLORS[row.status] ?? "#6B7280"}
        />
      ) : null}
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
        className={`mb-1 text-xs font-semibold tracking-wide uppercase ${accent}`}
      >
        {title} ({count})
      </h3>
      <ul className="divide-y divide-gray-100 rounded-2xl border border-gray-200 bg-white p-2 shadow-sm">
        {rows.map((row) => (
          <StudentRow key={row.student_id} row={row} />
        ))}
      </ul>
    </div>
  );
}

export default function KlassenPage() {
  const [classes, setClasses] = useState<string[] | null>(null);
  const [selectedClass, setSelectedClass] = useState<string>("");
  const [dateISO, setDateISO] = useState<string>(() => berlinTodayISO());
  const [report, setReport] = useState<ClassDayReport | null>(null);
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

  useEffect(() => {
    if (!selectedClass) {
      if (classes !== null) setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetchClassDay(selectedClass, dateISO)
      .then((response) => {
        if (!cancelled) setReport(response);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setReport(null);
        setError("Die Klassenansicht konnte nicht geladen werden.");
        logger.error("class_day_fetch_failed", {
          school_class: selectedClass,
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
  }, [selectedClass, dateISO, classes]);

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
              Übergabe nach Unterricht
            </h2>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
              Wer bleibt heute in Randstunde oder Ganztag, wer geht nach Hause –
              für Ihre Klasse am {formatDate(dateISO)}.
            </p>
          </div>
          <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
            <div className="flex items-center gap-1">
              <button
                type="button"
                aria-label="Vorheriger Tag"
                onClick={() => shiftDay(-1)}
                className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-600 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <ChevronLeft className="h-4 w-4" aria-hidden="true" />
              </button>
              <DatePicker
                value={selectedDate}
                onChange={(date) => {
                  if (date) setDateISO(toISODate(date));
                }}
                calendarLayout="popover"
                hideClearButton
                className="w-full sm:w-44"
              />
              <button
                type="button"
                aria-label="Nächster Tag"
                onClick={() => shiftDay(1)}
                className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-600 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <ChevronRight className="h-4 w-4" aria-hidden="true" />
              </button>
            </div>
          </div>
        </div>

        {classes !== null && classes.length > 1 && (
          <div className="mt-4">
            <SegmentedControl
              variant="joined"
              ariaLabel="Klasse wählen"
              value={selectedClass || null}
              onChange={setSelectedClass}
              items={classes.map((klass) => ({
                value: klass,
                label: `Klasse ${klass}`,
              }))}
            />
          </div>
        )}

        {noClasses && (
          <EmptyState
            className="mt-4"
            title="Keine Klassen zugewiesen"
            description="Ihrem Konto ist noch keine Klasse zugeordnet. Die OGS-Verwaltung kann Ihnen Klassen unter Mitarbeitende zuweisen."
          />
        )}

        {!noClasses && loading && (
          <div className="mt-4 space-y-3">
            <Skeleton className="h-16 w-full" />
            <Skeleton className="h-40 w-full" />
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
            <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-4">
              <Stat label="Klassenverband" value={report.totals.students} />
              <Stat label="Bleiben in der OGS" value={report.totals.staying} />
              <Stat label="Gehen nach Hause" value={report.totals.leaving} />
              <Stat label="Abgemeldet" value={report.totals.absent} />
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
              <div className="mt-4 space-y-4">
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

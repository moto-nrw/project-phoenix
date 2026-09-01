"use client";

// Statistik (#2606): attendance and absence quotas per child, group and
// period plus room utilization over the same window. Everything shown here
// is read-only and derived from what check-in, sick notes and room visits
// already record.
//
// Design follows the Anmeldungen/Planung surface language (same as the
// Tagesauswertung): one calm content section with an uppercase kicker,
// gray-50 stat blocks, plain tables, quiet white export buttons.

import { Download, FileSpreadsheet, FileText } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import type { DateRange } from "react-day-picker";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import {
  buildDefaultPresets,
  DateRangePicker,
} from "~/components/ui/date-range-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { MultiSelect } from "~/components/ui/multi-select";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { Skeleton } from "~/components/ui/skeleton";
import { StatCard } from "~/components/ui/stat-card";
import {
  berlinTodayISO,
  formatDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  fetchStatisticsReport,
  formatHours,
  formatRate,
  EXPORT_FILENAME_STEM,
  StatisticsError,
  statisticsExportUrl,
  type StatisticsCourseRow,
  type StatisticsCourseStudentRow,
  type StatisticsErrorCode,
  type StatisticsExportFormat,
  type StatisticsExportSection,
  type StatisticsGroupRow,
  type StatisticsReport,
  type StatisticsRoomRow,
  type StatisticsStudentRow,
} from "~/lib/statistics-api";
import { useTenantAwarePath } from "~/lib/tenant-path";

const logger = createLogger({ component: "StatisticsPage" });

type StatisticsView = "groups" | "students" | "rooms" | "courses";

const VIEW_ITEMS: readonly { value: StatisticsView; label: string }[] = [
  { value: "groups", label: "Gruppen" },
  { value: "students", label: "Kinder" },
  { value: "rooms", label: "Räume" },
  { value: "courses", label: "Kurse" },
];

// Der Kursbereich hat zwei Sichten auf dieselben Zahlen. Sie stehen als
// eigener Umschalter unter dem Bereichswechsel, damit oben vier Bereiche
// bleiben und die Leiste auf dem Handy lesbar ist.
type CourseView = "by-course" | "by-child";

const COURSE_VIEW_ITEMS: readonly { value: CourseView; label: string }[] = [
  { value: "by-course", label: "Je Kurs" },
  { value: "by-child", label: "Je Kind" },
];

const ERROR_MESSAGES: Record<StatisticsErrorCode, string> = {
  forbidden:
    "Ihr Konto darf die Statistik nicht sehen. Bitte wenden Sie sich an Ihre Administration.",
  invalid_request:
    "Der Zeitraum ist ungültig. Er darf höchstens ein Jahr umfassen und nicht in der Zukunft enden.",
  unknown: "Die Statistik konnte nicht geladen werden.",
};

function addDays(d: Date, days: number): Date {
  const r = new Date(d);
  r.setDate(r.getDate() + days);
  return r;
}

export default function StatisticsPage() {
  const tenantPath = useTenantAwarePath();
  const isMobile = useIsMobile();
  const todayISO = berlinTodayISO();
  const today = useMemo(() => parseISODate(todayISO), [todayISO]);
  const [range, setRange] = useState<DateRange | undefined>(() => ({
    from: addDays(parseISODate(berlinTodayISO()), -29),
    to: parseISODate(berlinTodayISO()),
  }));
  const [groupIds, setGroupIds] = useState<string[]>([]);
  const [availableGroups, setAvailableGroups] = useState<
    readonly StatisticsGroupRow[]
  >([]);
  const [view, setView] = useState<StatisticsView>("groups");
  const [courseView, setCourseView] = useState<CourseView>("by-course");
  const [data, setData] = useState<StatisticsReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [errorCode, setErrorCode] = useState<StatisticsErrorCode | null>(null);
  const [exporting, setExporting] = useState<string | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);

  const groupOptions = useMemo(
    () =>
      availableGroups
        .map((group) => ({ value: group.group_id, label: group.name }))
        .sort((a, b) => a.label.localeCompare(b.label, "de")),
    [availableGroups],
  );

  const fromISO = range?.from ? toISODate(range.from) : null;
  const toISO = range?.to ? toISODate(range.to) : null;

  useEffect(() => {
    if (!fromISO || !toISO) return;
    let cancelled = false;
    setLoading(true);
    setErrorCode(null);
    fetchStatisticsReport(fromISO, toISO, groupIds)
      .then((report) => {
        if (cancelled) return;
        setData(report);
        if (groupIds.length === 0) setAvailableGroups(report.groups);
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setData(null);
        setErrorCode(error instanceof StatisticsError ? error.code : "unknown");
        logger.error("statistics_fetch_failed", {
          from: fromISO,
          to: toISO,
          error: error instanceof Error ? error.message : String(error),
        });
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [fromISO, toISO, groupIds]);

  const downloadExport = useCallback(
    async (
      format: StatisticsExportFormat,
      section: StatisticsExportSection,
    ) => {
      if (!fromISO || !toISO) return;
      setExporting(`${section}-${format}`);
      setExportError(null);
      try {
        const res = await fetch(
          statisticsExportUrl(fromISO, toISO, format, groupIds, section),
        );
        if (!res.ok) {
          setExportError("Export fehlgeschlagen. Bitte erneut versuchen.");
          return;
        }
        const blob = await res.blob();
        const disposition = res.headers.get("Content-Disposition") ?? "";
        const filename =
          /filename="([^"]+)"/.exec(disposition)?.[1] ??
          `${EXPORT_FILENAME_STEM[section]}-${fromISO}-${toISO}.${format}`;
        const url = URL.createObjectURL(blob);
        const anchor = document.createElement("a");
        anchor.href = url;
        anchor.download = filename;
        document.body.appendChild(anchor);
        anchor.click();
        anchor.remove();
        URL.revokeObjectURL(url);
      } catch (error) {
        logger.error("statistics_export_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        setExportError("Export fehlgeschlagen. Bitte erneut versuchen.");
      } finally {
        setExporting(null);
      }
    },
    [fromISO, toISO, groupIds],
  );

  // Anchor 365 days back: the backend allows at most 366 days inclusive,
  // so the "Gesamt" preset must not overshoot the window by one day.
  const presets = useMemo(
    () => buildDefaultPresets(addDays(today, -365), today),
    [today],
  );

  const groupColumns: DataTableColumn<StatisticsGroupRow>[] = [
    {
      key: "name",
      header: "Gruppe",
      render: (row) => (
        <span className="font-medium text-gray-900">{row.name}</span>
      ),
      // The pseudo group of children without a group (id 0) sorts last.
      sortValue: (row) => (row.group_id === "0" ? "\uffff" : row.name),
    },
    {
      key: "students",
      header: "Kinder",
      align: "right",
      render: (row) => row.student_count,
      sortValue: (row) => row.student_count,
    },
    {
      key: "present",
      header: "Anwesend",
      align: "right",
      render: (row) => row.present_days,
      sortValue: (row) => row.present_days,
    },
    {
      key: "sick",
      header: "Krank",
      align: "right",
      render: (row) => row.sick_days,
      sortValue: (row) => row.sick_days,
    },
    {
      key: "excused",
      header: "Entschuldigt",
      align: "right",
      render: (row) => row.excused_days,
      sortValue: (row) => row.excused_days,
    },
    {
      key: "unexplained",
      header: "Ohne Meldung",
      align: "right",
      render: (row) => (
        <span
          className={
            row.unexplained_days > 0 ? "text-moto-red-strong" : undefined
          }
        >
          {row.unexplained_days}
        </span>
      ),
      sortValue: (row) => row.unexplained_days,
    },
    {
      key: "rate",
      header: "Quote",
      align: "right",
      render: (row) => (
        <span className="font-medium text-gray-900">
          {formatRate(row.attendance_rate)}
        </span>
      ),
      sortValue: (row) => row.attendance_rate ?? -1,
    },
  ];

  const studentColumns: DataTableColumn<StatisticsStudentRow>[] = [
    {
      key: "name",
      header: "Kind",
      render: (row) => (
        <Link
          href={tenantPath(`/students/${row.student_id}`)}
          className="font-medium text-gray-900 hover:underline"
        >
          {row.last_name}, {row.first_name}
        </Link>
      ),
      sortValue: (row) => `${row.last_name} ${row.first_name}`,
    },
    {
      key: "class",
      header: "Klasse",
      render: (row) => row.school_class,
      sortValue: (row) => row.school_class,
    },
    {
      key: "group",
      header: "Gruppe",
      render: (row) => row.group_name || "Ohne Gruppe",
      sortValue: (row) => row.group_name,
    },
    {
      key: "present",
      header: "Anwesend",
      align: "right",
      render: (row) => row.present_days,
      sortValue: (row) => row.present_days,
    },
    {
      key: "sick",
      header: "Krank",
      align: "right",
      render: (row) => row.sick_days,
      sortValue: (row) => row.sick_days,
    },
    {
      key: "excused",
      header: "Entschuldigt",
      align: "right",
      render: (row) => row.excused_days,
      sortValue: (row) => row.excused_days,
    },
    {
      key: "unexplained",
      header: "Ohne Meldung",
      align: "right",
      render: (row) => (
        <span
          className={
            row.unexplained_days > 0 ? "text-moto-red-strong" : undefined
          }
        >
          {row.unexplained_days}
        </span>
      ),
      sortValue: (row) => row.unexplained_days,
    },
    {
      key: "rate",
      header: "Quote",
      align: "right",
      render: (row) => (
        <span className="font-medium text-gray-900">
          {formatRate(row.attendance_rate)}
        </span>
      ),
      sortValue: (row) => row.attendance_rate ?? -1,
    },
  ];

  const roomColumns: DataTableColumn<StatisticsRoomRow>[] = [
    {
      key: "name",
      header: "Raum",
      render: (row) => (
        <span className="font-medium text-gray-900">{row.name}</span>
      ),
      sortValue: (row) => row.name,
    },
    {
      key: "capacity",
      header: "Plätze",
      align: "right",
      render: (row) => row.capacity ?? "",
      sortValue: (row) => row.capacity ?? -1,
    },
    {
      key: "days",
      header: "Tage genutzt",
      align: "right",
      render: (row) => row.days_used,
      sortValue: (row) => row.days_used,
    },
    {
      key: "students",
      header: "Kinder",
      align: "right",
      render: (row) => row.distinct_students,
      sortValue: (row) => row.distinct_students,
    },
    {
      key: "hours",
      header: "Stunden",
      align: "right",
      render: (row) => formatHours(row.student_minutes),
      sortValue: (row) => row.student_minutes,
    },
    {
      key: "peak",
      header: "Spitze",
      align: "right",
      render: (row) => row.peak_occupancy,
      sortValue: (row) => row.peak_occupancy,
    },
    {
      key: "utilization",
      header: "Auslastung",
      align: "right",
      render: (row) => (
        <span className="font-medium text-gray-900">
          {formatRate(row.peak_utilization_percent)}
        </span>
      ),
      sortValue: (row) => row.peak_utilization_percent ?? -1,
    },
  ];

  const courseColumns: DataTableColumn<StatisticsCourseRow>[] = [
    {
      key: "name",
      header: "Kurs",
      render: (row) => (
        <div>
          <div className="font-medium text-gray-900">{row.name}</div>
          {row.category_name && (
            <div className="text-xs text-gray-500">{row.category_name}</div>
          )}
        </div>
      ),
      sortValue: (row) => row.name,
    },
    {
      key: "held",
      header: "Termine",
      align: "right",
      render: (row) => row.held_instances,
      sortValue: (row) => row.held_instances,
    },
    {
      key: "cancelled",
      header: "Abgesagt",
      align: "right",
      render: (row) => row.cancelled_instances,
      sortValue: (row) => row.cancelled_instances,
    },
    {
      key: "children",
      header: "Kinder",
      align: "right",
      render: (row) => row.student_count,
      sortValue: (row) => row.student_count,
    },
    {
      key: "seats",
      header: "Plätze",
      align: "right",
      render: (row) =>
        row.max_participants > 0 ? row.max_participants : "unbegrenzt",
      sortValue: (row) => row.max_participants,
    },
    {
      key: "present",
      header: "Teilnahme",
      align: "right",
      render: (row) => row.present_days,
      sortValue: (row) => row.present_days,
    },
    {
      key: "absent",
      header: "Fehltage",
      align: "right",
      render: (row) => row.absent_days,
      sortValue: (row) => row.absent_days,
    },
    {
      key: "open",
      header: "Offen",
      align: "right",
      render: (row) => row.open_days,
      sortValue: (row) => row.open_days,
    },
    {
      key: "rate",
      header: "Quote",
      align: "right",
      render: (row) => (
        <span className="font-medium text-gray-900">
          {formatRate(row.participation_rate)}
        </span>
      ),
      sortValue: (row) => row.participation_rate ?? -1,
    },
    {
      key: "occupancy",
      header: "Belegung",
      align: "right",
      render: (row) => formatRate(row.occupancy_percent),
      sortValue: (row) => row.occupancy_percent ?? -1,
    },
  ];

  const courseStudentColumns: DataTableColumn<StatisticsCourseStudentRow>[] = [
    {
      key: "name",
      header: "Kind",
      render: (row) => (
        <Link
          href={tenantPath(`/students/${row.student_id}`)}
          className="font-medium text-gray-900 hover:underline"
        >
          {row.last_name}, {row.first_name}
        </Link>
      ),
      sortValue: (row) => `${row.last_name} ${row.first_name}`,
    },
    {
      key: "class",
      header: "Klasse",
      render: (row) => row.school_class,
      sortValue: (row) => row.school_class,
    },
    {
      key: "course",
      header: "Kurs",
      render: (row) => row.course_name,
      sortValue: (row) => row.course_name,
    },
    {
      key: "present",
      header: "Teilnahme",
      align: "right",
      render: (row) => row.present_days,
      sortValue: (row) => row.present_days,
    },
    {
      key: "absent",
      header: "Fehltage",
      align: "right",
      render: (row) => row.absent_days,
      sortValue: (row) => row.absent_days,
    },
    {
      key: "open",
      header: "Offen",
      align: "right",
      render: (row) => row.open_days,
      sortValue: (row) => row.open_days,
    },
    {
      key: "rate",
      header: "Quote",
      align: "right",
      render: (row) => (
        <span className="font-medium text-gray-900">
          {formatRate(row.participation_rate)}
        </span>
      ),
      sortValue: (row) => row.participation_rate ?? -1,
    },
  ];

  // Export trio in the Anmeldungen button idiom: quiet white bordered
  // actions. The child table and the room table are separate documents
  // (different columns), so each section carries its own trio.
  const exportButtons = (section: StatisticsExportSection) => {
    const formats: {
      format: StatisticsExportFormat;
      label: string;
      Icon: typeof Download;
    }[] = [
      { format: "pdf", label: "PDF", Icon: Download },
      { format: "xlsx", label: "Excel", Icon: FileSpreadsheet },
      { format: "docx", label: "Word", Icon: FileText },
    ];
    return formats.map(({ format, label, Icon }) => (
      <Button
        key={format}
        type="button"
        variant="outline"
        size="md"
        className="gap-2 bg-white"
        disabled={!data || exporting !== null}
        onClick={() => void downloadExport(format, section)}
      >
        <Icon className="h-4 w-4" aria-hidden />
        {exporting === `${section}-${format}` ? "Wird exportiert…" : label}
      </Button>
    ));
  };

  const courseDataStartsInsideWindow =
    data !== null && fromISO !== null && data.course_data_from > fromISO;
  const courseDataAllBeforeWindow =
    data !== null && toISO !== null && data.course_data_from > toISO;

  const roomDataStartsInsideWindow =
    data !== null && fromISO !== null && data.room_data_from > fromISO;
  const roomDataAllBeforeWindow =
    data !== null && toISO !== null && data.room_data_from > toISO;

  // Überschrift des sichtbaren Bereichs. Die drei Bereiche stehen nie
  // gleichzeitig auf dem Schirm, deshalb reicht eine Zeile Markup unter dem
  // Umschalter statt einer eigenen Komponente.
  const sectionHeading: Readonly<{ title: string; hint: string }> = {
    groups: {
      title: "Gruppen",
      hint: "Kinder zählen in ihrer heutigen Gruppe. Die Quote teilt alle Anwesenheitstage durch alle Betreuungstage.",
    },
    students: {
      title: "Kinder",
      hint: "Zahlen sind Tage. Ein Klick auf den Namen öffnet die Detailseite des Kindes.",
    },
    courses: {
      title: "Kurse",
      hint:
        courseView === "by-course"
          ? "Eine Zeile je Kurs. Offen sind Termine, die noch nicht abgeschlossen wurden; sie verändern die Quote nicht. Nur zur Information, hier wird nichts geändert."
          : "Eine Zeile je Kind und Kurs. Offen sind Termine, die noch nicht abgeschlossen wurden. Ein Klick auf den Namen öffnet die Detailseite des Kindes.",
    },
    rooms: {
      title: "Räume",
      hint: data
        ? `Raumdaten können höchstens ${data.room_data_days} Tage zurückreichen (ab ${formatDate(data.room_data_from)}). Je Kind kann die Frist kürzer sein.`
        : "",
    },
  }[view];

  return (
    <div className="w-full">
      <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
              Statistik
            </p>
            <h2 className="mt-1 text-base font-semibold text-gray-900">
              Anwesenheit, Räume und Kurse im Zeitraum
            </h2>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
              {view === "courses"
                ? "Quote = Teilnahmetage geteilt durch die Termine, für die eine Teilnahme entschieden wurde. Abgesagte Termine zählen nicht mit."
                : "Quote = Tage mit Anmeldung geteilt durch Betreuungstage (ohne Feiertage, Schließtage und Ferien)."}
            </p>
          </div>
          <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-end lg:shrink-0">
            <DateRangePicker
              value={range}
              onChange={(next) => {
                if (next?.from && next?.to) setRange(next);
              }}
              presets={presets}
              toMax={today}
              className="w-full sm:w-auto"
              triggerClassName="w-full justify-center sm:w-auto sm:justify-start"
            />
            <MultiSelect
              value={groupIds}
              options={groupOptions}
              onChange={setGroupIds}
              ariaLabel="Gruppen filtern"
              placeholder="Alle Gruppen"
              summaryLabel={(n) => `${n} Gruppen`}
              className="w-full sm:w-44"
            />
            <div className="flex flex-wrap gap-2 sm:justify-end">
              {exportButtons("attendance")}
            </div>
          </div>
        </div>

        {exportError && (
          <div className="mt-4">
            <Alert type="error" message={exportError} />
          </div>
        )}

        {loading && (
          <div className="mt-4 space-y-3">
            <Skeleton className="h-16 w-full" />
            <Skeleton className="h-40 w-full" />
            <Skeleton className="h-40 w-full" />
          </div>
        )}

        {!loading && errorCode !== null && (
          <EmptyState
            className="mt-4"
            title="Statistik nicht verfügbar"
            description={ERROR_MESSAGES[errorCode]}
          />
        )}

        {!loading && errorCode === null && data && (
          <>
            {/* Die Kennzahlen wechseln mit dem Bereich: eine Quote über
                Betreuungstagen neben einer Kurstabelle wäre die falsche Zahl
                zur falschen Tabelle. */}
            {view === "courses" ? (
              <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-6">
                <StatCard
                  variant="tile"
                  label="Kurse"
                  value={data.courses.length}
                />
                <StatCard
                  variant="tile"
                  label="Termine"
                  value={data.course_totals.held_instances}
                />
                <StatCard
                  variant="tile"
                  label="Abgesagt"
                  value={data.course_totals.cancelled_instances}
                />
                <StatCard
                  variant="tile"
                  label="Kinder"
                  value={data.course_totals.student_count}
                />
                <StatCard
                  variant="tile"
                  label="Quote gesamt"
                  // Ohne entschiedenen Termin gibt es keine Quote. Ein
                  // Gedankenstrich sagt das; ein leeres Feld sieht kaputt aus.
                  value={
                    formatRate(data.course_totals.participation_rate) || "–"
                  }
                />
                <StatCard
                  variant="tile"
                  label="Offen"
                  value={data.course_totals.open_days}
                />
              </div>
            ) : (
              <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-4 lg:grid-cols-7">
                <StatCard
                  variant="tile"
                  label="Betreuungstage"
                  value={data.care_days}
                />
                <StatCard
                  variant="tile"
                  label="Tage abgezogen"
                  value={data.excluded_days.total}
                />
                <StatCard
                  variant="tile"
                  label="Kinder"
                  value={data.totals.student_count}
                />
                <StatCard
                  variant="tile"
                  label="Quote gesamt"
                  value={formatRate(data.totals.attendance_rate)}
                />
                <StatCard
                  variant="tile"
                  label="Krank"
                  value={data.totals.sick_days}
                />
                <StatCard
                  variant="tile"
                  label="Entschuldigt"
                  value={data.totals.excused_days}
                />
                {/* Rot, sobald es offene Fälle gibt: die Zahl ist die einzige
                  auf dem Schirm, der jemand nachgehen muss. */}
                <StatCard
                  variant="tile"
                  label="Ohne Meldung"
                  value={data.totals.unexplained_days}
                  tone={data.totals.unexplained_days > 0 ? "red" : undefined}
                />
              </div>
            )}
            <p className="mt-2 text-xs leading-5 text-gray-500">
              {view === "courses" ? (
                <>
                  {formatDate(data.from)} bis {formatDate(data.to)} · gezählt
                  werden nur Termine, die stattgefunden haben
                </>
              ) : (
                <>
                  {formatDate(data.from)} bis {formatDate(data.to)} · abgezogen:{" "}
                  {data.excluded_days.public_holidays} Feiertage,{" "}
                  {data.excluded_days.closing_days} Schließtage,{" "}
                  {data.excluded_days.holiday_periods} Ferientage
                </>
              )}
            </p>

            <div className="mt-5 flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
              {/* Mobile fills the line, desktop keeps the compact joined chip. */}
              <SegmentedControl
                items={VIEW_ITEMS}
                value={view}
                onChange={setView}
                variant="joined"
                fullWidth={isMobile}
                ariaLabel="Bereich wählen"
              />
              {view === "rooms" && !roomDataAllBeforeWindow && (
                <div className="flex flex-wrap gap-2">
                  {exportButtons("rooms")}
                </div>
              )}
              {view === "courses" && !courseDataAllBeforeWindow && (
                <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
                  <SegmentedControl
                    items={COURSE_VIEW_ITEMS}
                    value={courseView}
                    onChange={setCourseView}
                    variant="pills"
                    ariaLabel="Kurs-Sicht wählen"
                  />
                  <div className="flex flex-wrap gap-2">
                    {exportButtons(
                      courseView === "by-course"
                        ? "courses"
                        : "course-students",
                    )}
                  </div>
                </div>
              )}
            </div>

            <div className="mt-3">
              <div className="mb-3">
                <h3 className="text-sm font-semibold text-gray-900">
                  {sectionHeading.title}
                </h3>
                <p className="text-xs leading-5 text-gray-500">
                  {sectionHeading.hint}
                </p>
              </div>

              {view === "groups" && (
                <DataTable
                  columns={groupColumns}
                  rows={data.groups}
                  getRowKey={(row) => row.group_id}
                  defaultSortKey="name"
                  emptyState={
                    <EmptyState
                      title="Keine Kinder im Zeitraum"
                      description="Für die gewählten Gruppen gibt es keine Kinder."
                    />
                  }
                />
              )}

              {view === "students" && (
                <DataTable
                  columns={studentColumns}
                  rows={data.students}
                  getRowKey={(row) => row.student_id}
                  defaultSortKey="name"
                  pageSize={25}
                  paginationResetKey={`${fromISO}-${toISO}-${groupIds.join(",")}`}
                  emptyState={
                    <EmptyState
                      title="Keine Kinder im Zeitraum"
                      description="Für die gewählten Gruppen gibt es keine Kinder."
                    />
                  }
                />
              )}

              {view === "courses" &&
                (courseDataAllBeforeWindow ? (
                  <EmptyState
                    title="Keine Kurstermine für diesen Zeitraum"
                    description={`Termine werden ${data.course_data_days} Tage aufbewahrt. Wählen Sie einen Zeitraum ab ${formatDate(data.course_data_from)}.`}
                  />
                ) : (
                  <>
                    {courseDataStartsInsideWindow && (
                      <div className="mb-3">
                        <Alert
                          type="info"
                          message={`Kurstermine werden ${data.course_data_days} Tage aufbewahrt. Ältere Termine als ${formatDate(data.course_data_from)} sind nicht mehr gespeichert.`}
                        />
                      </div>
                    )}
                    {courseView === "by-course" ? (
                      <DataTable
                        columns={courseColumns}
                        rows={data.courses}
                        getRowKey={(row) => row.course_id}
                        defaultSortKey="name"
                        pageSize={25}
                        paginationResetKey={`${fromISO}-${toISO}-${groupIds.join(",")}`}
                        emptyState={
                          <EmptyState
                            title="Keine Kurse im Zeitraum"
                            description="Im gewählten Zeitraum gab es keine Kurstermine. Termine entstehen aus dem Betreuungsplan."
                          />
                        }
                      />
                    ) : (
                      <DataTable
                        columns={courseStudentColumns}
                        rows={data.course_students}
                        getRowKey={(row) =>
                          `${row.student_id}-${row.course_id}`
                        }
                        defaultSortKey="name"
                        pageSize={25}
                        paginationResetKey={`${fromISO}-${toISO}-${groupIds.join(",")}`}
                        emptyState={
                          <EmptyState
                            title="Keine Teilnahme im Zeitraum"
                            description="Für die gewählten Gruppen gibt es keine Kursteilnahme."
                          />
                        }
                      />
                    )}
                  </>
                ))}

              {view === "rooms" &&
                (roomDataAllBeforeWindow ? (
                  <EmptyState
                    title="Keine Raumdaten für diesen Zeitraum"
                    description={`Wählen Sie einen Zeitraum ab ${formatDate(data.room_data_from)}.`}
                  />
                ) : (
                  <>
                    {roomDataStartsInsideWindow && (
                      <div className="mb-3">
                        <Alert
                          type="info"
                          message={`Raumdaten können erst ab ${formatDate(data.room_data_from)} vorhanden sein. Je Kind kann die Frist kürzer sein.`}
                        />
                      </div>
                    )}
                    <DataTable
                      columns={roomColumns}
                      rows={data.rooms}
                      getRowKey={(row) => row.room_id}
                      defaultSortKey="days"
                      defaultSortDirection="desc"
                      emptyState={
                        <EmptyState
                          title="Keine Räume"
                          description="Es sind keine Räume angelegt."
                        />
                      }
                    />
                  </>
                ))}
            </div>
          </>
        )}
      </section>
    </div>
  );
}

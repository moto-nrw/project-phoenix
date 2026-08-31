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
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import {
  buildDefaultPresets,
  DateRangePicker,
} from "~/components/ui/date-range-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { ForbiddenPage } from "~/components/ui/forbidden-page";
import { MultiSelect } from "~/components/ui/multi-select";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { OverflowMenuEntry } from "~/components/ui/page-header/OverflowMenu";
import { PlanningContextBar } from "~/components/ui/planning-context-bar";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { LOCATION_COLORS, getAccessibleTextColor } from "~/lib/location-helper";
import {
  berlinTodayISO,
  formatDate,
  formatStatusDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  fetchStatisticsReport,
  formatHours,
  formatRate,
  StatisticsError,
  statisticsExportUrl,
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

type StatisticsView = "groups" | "students" | "rooms";

const VIEW_ITEMS: readonly { value: StatisticsView; label: string }[] = [
  { value: "groups", label: "Gruppen" },
  { value: "students", label: "Kinder" },
  { value: "rooms", label: "Räume" },
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
          `${section === "rooms" ? "raumauslastung" : "statistik"}-${fromISO}-${toISO}.${format}`;
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

  // Zeitnavigation der Bedienleiste: die Pfeile verschieben das gewählte
  // Fenster um seine eigene Länge, "Letzte 30 Tage" stellt den Startzustand
  // der Seite wieder her. Beides ändert nur den Zeitraum, sonst nichts.
  const windowDays =
    range?.from && range?.to
      ? Math.round(
          (range.to.getTime() - range.from.getTime()) / (24 * 60 * 60 * 1000),
        ) + 1
      : null;

  const shiftWindow =
    range?.from && range?.to && windowDays
      ? (direction: number) => {
          const from = range.from;
          const to = range.to;
          if (!from || !to) return;
          let nextTo = addDays(to, direction * windowDays);
          // Der Bericht endet nie in der Zukunft: beim Vorwärtsschieben rückt
          // das Fenster höchstens bis heute, statt einen ungültigen Zeitraum
          // anzufragen.
          if (nextTo > today) nextTo = today;
          const shift = Math.round(
            (nextTo.getTime() - to.getTime()) / (24 * 60 * 60 * 1000),
          );
          if (shift === 0) return;
          setRange({ from: addDays(from, shift), to: nextTo });
        }
      : null;

  const canShiftForward = toISO !== null && toISO < todayISO;

  const defaultFromISO = toISODate(addDays(today, -29));
  const isOnDefaultWindow = fromISO === defaultFromISO && toISO === todayISO;

  const resetWindow = () => setRange({ from: addDays(today, -29), to: today });

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
            // Offene Fälle brauchen Aufmerksamkeit, sie sind aber kein
            // Fehler: Orange ist dafür die Farbe, Rot gehört Krank und
            // echten Fehlern (siehe Farbtabelle im UI-Kit).
            row.unexplained_days > 0 ? "text-moto-orange-strong" : undefined
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
            // Offene Fälle brauchen Aufmerksamkeit, sie sind aber kein
            // Fehler: Orange ist dafür die Farbe, Rot gehört Krank und
            // echten Fehlern (siehe Farbtabelle im UI-Kit).
            row.unexplained_days > 0 ? "text-moto-orange-strong" : undefined
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

  // Export liegt auf jeder anderen Seite im Kebab-Menü und nicht als
  // Knopfreihe in der Titelzeile. Drei Formate ergaben dort drei Knöpfe, die
  // allein die halbe Kopfzeile füllten. Kind- und Raumtabelle sind
  // verschiedene Dokumente, deshalb bekommt jeder Bereich seinen eigenen
  // Menüeintrag.
  const exportMenuItems = (
    section: StatisticsExportSection,
  ): OverflowMenuEntry[] => [
    { kind: "header", label: "Exportieren" },
    ...(
      [
        { format: "pdf", label: "PDF", Icon: Download },
        { format: "xlsx", label: "Excel", Icon: FileSpreadsheet },
        { format: "docx", label: "Word", Icon: FileText },
      ] as {
        format: StatisticsExportFormat;
        label: string;
        Icon: typeof Download;
      }[]
    ).map(({ format, label, Icon }) => ({
      label: exporting === `${section}-${format}` ? "Wird exportiert…" : label,
      icon: <Icon className="h-4 w-4" aria-hidden />,
      onClick: () => void downloadExport(format, section),
      disabled: !data || exporting !== null,
    })),
  ];

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
    rooms: {
      title: "Räume",
      hint: data
        ? `Raumdaten können höchstens ${data.room_data_days} Tage zurückreichen (ab ${formatDate(data.room_data_from)}). Je Kind kann die Frist kürzer sein.`
        : "",
    },
  }[view];

  // Statuszeile unter dem Titel: echter Zeitraum, gezählte Betreuungstage und
  // Kinder aus dem geladenen Bericht.
  // Kein Zeitraum in der Statuszeile: den trägt die Zeitraumwahl direkt
  // darunter. Zweimal dieselben Daten in der Kopfkarte kosteten auf dem
  // Telefon eine Zeile, die nichts sagte.
  const statusLine = data
    ? `${data.care_days} Betreuungstage · ${data.totals.student_count} Kinder`
    : formatStatusDate(todayISO);

  // Fehlendes Recht ist ein Zustand, kein Fehler: eigener ruhiger Leerzustand
  // statt einer roten Fehlermeldung (Querregel "Zustände").
  if (errorCode === "forbidden") {
    return (
      <ForbiddenPage title="Statistik" message={ERROR_MESSAGES.forbidden} />
    );
  }

  return (
    <TenantPage
      title="Statistik"
      stats={statusLine}
      statsLoading={loading}
      loading={loading}
      error={errorCode !== null ? ERROR_MESSAGES[errorCode] : null}
      tabs={{
        value: view,
        onChange: (next) => setView(next as StatisticsView),
        items: VIEW_ITEMS,
        label: "Bereich wählen",
      }}
      actions={
        <OverflowMenu
          items={exportMenuItems("attendance")}
          ariaLabel="Weitere Aktionen"
        />
      }
      // Bauart 3, Regel 1: die Zeitnavigation sitzt im Bedienband der
      // Kopfkarte, nicht als Bedienelement neben dem Titel. Der Zeitraum ist
      // hier frei wählbar, deshalb steht der Bereichswähler an der Stelle des
      // Wochenetiketts; die Pfeile schieben das gewählte Fenster um seine
      // eigene Länge weiter.
      searchSlot={
        <PlanningContextBar
          withoutContextRow
          navigationInGroup
          navigationSlot={
            <DateRangePicker
              value={range}
              onChange={(next) => {
                if (next?.from && next?.to) setRange(next);
              }}
              presets={presets}
              toMax={today}
              className="min-w-0"
              // Zwischen den Pfeilen: Rahmen und Rundung kommen von der
              // Gruppe, der Chip bringt nur Inhalt und Höhe mit.
              triggerClassName="h-9 w-full justify-center rounded-none border-0 px-3"
            />
          }
          onPrevious={shiftWindow ? () => shiftWindow(-1) : undefined}
          onNext={
            canShiftForward && shiftWindow ? () => shiftWindow(1) : undefined
          }
          previousLabel="Vorheriger Zeitraum"
          nextLabel="Nächster Zeitraum"
          onToday={isOnDefaultWindow ? undefined : resetWindow}
          todayLabel="Letzte 30 Tage"
          actions={
            <MultiSelect
              ariaLabel="Gruppen"
              value={groupIds}
              options={groupOptions}
              onChange={setGroupIds}
              placeholder="Alle Gruppen"
              summaryLabel={(count) => `${count} Gruppen`}
              className="w-full md:w-56"
              triggerClassName="moto-content-surface h-9 w-full hover:border-gray-300"
            />
          }
        >
          {/* Kontextzeile: nur was NICHT schon in der Statuszeile steht.
              Ohne Gruppenfilter bleibt sie still. */}
          {groupIds.length > 0 && (
            <span>
              Gefiltert auf {groupIds.length} von {groupOptions.length} Gruppen
            </span>
          )}
        </PlanningContextBar>
      }
    >
      {exportError && <Alert type="error" message={exportError} />}

      {data && (
        <>
          {/* Kennzahlen des Zeitraums als eigene Karte; die Tabelle darunter
              ist ihre eigene. Karten stapeln sich nicht ineinander. */}
          {/* EINE große Aussage statt sieben gleichförmiger Mini-Kacheln
              (Eltern-Portal-Muster: erst die eine Antwort, dann das Detail).
              Betreuungstage und Kinderzahl stehen schon in der Statuszeile
              der Kopfkarte, die abgezogenen Tage im Satz darunter — die
              frühere Kachelreihe hat drei ihrer sieben Werte dupliziert. */}
          <SectionCard>
            <div className="space-y-4">
              <div className="flex flex-wrap items-start gap-x-10 gap-y-4">
                <div>
                  <p className="text-xs font-semibold tracking-wider text-gray-500 uppercase">
                    Quote gesamt
                  </p>
                  <p className="mt-1 text-2xl leading-tight font-semibold text-gray-900 tabular-nums">
                    {formatRate(data.totals.attendance_rate)}
                  </p>
                </div>
                <dl className="flex flex-wrap items-start gap-x-10 gap-y-4">
                  {(
                    [
                      ["Krank", data.totals.sick_days, false],
                      ["Entschuldigt", data.totals.excused_days, false],
                      // Hervorgehoben, sobald es offene Fälle gibt: die Zahl
                      // ist die einzige auf dem Schirm, der jemand nachgehen
                      // muss.
                      [
                        "Ohne Meldung",
                        data.totals.unexplained_days,
                        data.totals.unexplained_days > 0,
                      ],
                    ] as const
                  ).map(([label, value, highlight]) => (
                    <div key={label}>
                      <dt className="text-xs font-semibold tracking-wider text-gray-500 uppercase">
                        {label}
                      </dt>
                      <dd
                        className="mt-1 text-lg leading-tight font-semibold tabular-nums"
                        style={{
                          color: highlight
                            ? getAccessibleTextColor(LOCATION_COLORS.SCHOOLYARD)
                            : undefined,
                        }}
                      >
                        {value}
                      </dd>
                    </div>
                  ))}
                </dl>
              </div>
              <p className="text-sm leading-6 text-gray-600">
                {formatDate(data.from)} bis {formatDate(data.to)} · abgezogen:{" "}
                {data.excluded_days.public_holidays} Feiertage,{" "}
                {data.excluded_days.closing_days} Schließtage,{" "}
                {data.excluded_days.holiday_periods} Ferientage
              </p>
            </div>
          </SectionCard>

          {/* Der sichtbare Unterabschnitt mit seiner Tabelle. */}
          <SectionCard
            title={sectionHeading.title}
            description={sectionHeading.hint}
            actions={
              view === "rooms" && !roomDataAllBeforeWindow ? (
                <OverflowMenu
                  items={exportMenuItems("rooms")}
                  ariaLabel="Raumtabelle exportieren"
                />
              ) : undefined
            }
          >
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
          </SectionCard>
        </>
      )}
    </TenantPage>
  );
}

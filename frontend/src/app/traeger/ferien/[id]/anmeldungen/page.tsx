"use client";

// Ferienanmeldungen eines Zeitraums prüfen (#4663): alle Anmeldungen mit
// Status, Schule, Woche und Zeitmodell, mit Filtern, Hinweis auf mögliche
// Doppelanmeldungen und Export als CSV oder Excel.

import { ClipboardList } from "lucide-react";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  childSchoolLabel,
  formatHolidayWeek,
  HOLIDAY_REGISTRATION_STATUS_LABELS,
  HOLIDAY_REGISTRATION_STATUS_TONES,
  holidayTimeModelNames,
  schoolNames,
} from "~/components/traeger/holiday-registration-format";
import { holidayPeriodTabs } from "~/components/traeger/holiday-period-head";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { Alert } from "~/components/ui/alert";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import {
  ExportMenu,
  type ExportMenuOption,
  exportFormatOptions,
} from "~/components/ui/export-menu";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiErrorDisplay, useApiLoadError } from "~/contexts/ToastContext";
import { formatBerlinDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import { useTraegerHolidayPeriod } from "~/components/traeger/use-traeger-resources";
import {
  downloadHolidayRegistrations,
  EMPTY_HOLIDAY_REGISTRATION_FILTER,
  fetchHolidayRegistrations,
  type HolidayRegistrationFilter,
  type HolidayRegistrationList,
  type HolidayRegistrationStatus,
  type HolidayRegistrationSummary,
} from "~/lib/traeger-holiday-registrations-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerHolidayRegistrationsPage" });

type StatusChoice = HolidayRegistrationStatus | "all";

const EXPORT_FORMATS: readonly ExportMenuOption[] = exportFormatOptions([
  "xlsx",
  "csv",
]);

const STATUS_OPTIONS: { value: StatusChoice; label: string }[] = [
  { value: "all", label: "Alle Status" },
  { value: "received", label: "Eingegangen" },
  { value: "confirmed", label: "Bestätigt" },
  { value: "cancelled", label: "Storniert" },
];

function columns(
  names: ReadonlyMap<string, string>,
): DataTableColumn<HolidayRegistrationSummary>[] {
  return [
    {
      key: "number",
      header: "Nr.",
      render: (registration) => registration.id,
      sortValue: (registration) => Number(registration.id),
      stacked: "meta",
    },
    {
      key: "guardian",
      header: "Eltern",
      render: (registration) => (
        <span className="font-medium text-gray-900">
          {registration.guardianFirstName} {registration.guardianLastName}
        </span>
      ),
      sortValue: (registration) => registration.guardianLastName,
      stacked: "title",
    },
    {
      key: "children",
      header: "Kinder und Wochen",
      render: (registration) => (
        <ul className="space-y-2">
          {registration.children.map((child) => (
            <li key={child.id}>
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-medium text-gray-900">
                  {child.firstName} {child.lastName}
                </span>
                <span className="text-gray-500">
                  {childSchoolLabel(child, names)}
                </span>
                {child.duplicateRegistrationIds.length > 0 ? (
                  <StatusBadge
                    tone="orange"
                    label="Mögliche Doppelanmeldung"
                    title={`Auch in Anmeldung Nr. ${child.duplicateRegistrationIds.join(", ")}`}
                  />
                ) : null}
              </div>
              <ul className="text-xs text-gray-500">
                {child.bookings.map((booking) => (
                  <li key={booking.weekId}>
                    {formatHolidayWeek(
                      booking.weekStartsOn,
                      booking.weekEndsOn,
                    )}
                    : {booking.timeModelName}
                  </li>
                ))}
              </ul>
            </li>
          ))}
        </ul>
      ),
      stacked: "field",
    },
    {
      key: "submittedAt",
      header: "Eingegangen am",
      render: (registration) => formatBerlinDate(registration.submittedAt),
      sortValue: (registration) => registration.submittedAt,
      stacked: "meta",
    },
    {
      key: "status",
      header: "Status",
      render: (registration) => (
        <div className="flex flex-wrap gap-1">
          <StatusBadge
            tone={HOLIDAY_REGISTRATION_STATUS_TONES[registration.status]}
            label={HOLIDAY_REGISTRATION_STATUS_LABELS[registration.status]}
          />
          {registration.frozen ? (
            <StatusBadge tone="gray" label="Eingefroren" />
          ) : null}
        </div>
      ),
      sortValue: (registration) => registration.status,
      stacked: "meta",
    },
  ];
}

function statusLine(list: HolidayRegistrationList): string {
  const total = list.total === 1 ? "1 Anmeldung" : `${list.total} Anmeldungen`;
  return `${total} · ${list.statusCounts.received} eingegangen · ${list.statusCounts.confirmed} bestätigt · ${list.statusCounts.cancelled} storniert`;
}

/** Registrations with a child that is maybe registered twice. */
function duplicateCount(list: HolidayRegistrationList | null): number {
  return (list?.registrations ?? []).filter((registration) =>
    registration.children.some(
      (child) => child.duplicateRegistrationIds.length > 0,
    ),
  ).length;
}

export default function TraegerHolidayRegistrationsPage() {
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const periodId = params?.id ?? "";
  const {
    data: period,
    preview,
    loadError: periodError,
  } = useTraegerHolidayPeriod(periodId);
  const [list, setList] = useState<HolidayRegistrationList | null>(null);
  const [status, setStatus] = useState<StatusChoice>("all");
  const [school, setSchool] = useState("");
  const [weekId, setWeekId] = useState("");
  const [timeModel, setTimeModel] = useState("");
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState<"csv" | "xlsx" | null>(null);
  // Each new filter starts a new list; an answer for an older one is dropped.
  const generation = useRef(0);
  const { schools, error: schoolsError } = useTraegerSchools();
  const listLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = listLoad;
  const { show: showActionError } = useApiErrorDisplay();

  const filter = useMemo<HolidayRegistrationFilter>(
    () => ({
      ...EMPTY_HOLIDAY_REGISTRATION_FILTER,
      status: status === "all" ? "" : status,
      school,
      weekId,
      timeModel,
    }),
    [status, school, weekId, timeModel],
  );

  const load = useCallback(async () => {
    const current = ++generation.current;
    setLoading(true);
    try {
      const loaded = await fetchHolidayRegistrations(periodId, filter);
      if (current !== generation.current) return;
      setList(loaded);
      clearLoadError();
    } catch (err) {
      if (current !== generation.current) return;
      logger.error("traeger_holiday_registrations_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Ferienanmeldungen",
        retry: () => void load(),
      });
    } finally {
      if (current === generation.current) setLoading(false);
    }
  }, [periodId, filter, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const exportFile = async (format: "csv" | "xlsx") => {
    setExporting(format);
    try {
      const file = await downloadHolidayRegistrations(periodId, filter, format);
      downloadBlob(file.blob, file.filename ?? `ferienanmeldungen.${format}`);
    } catch (err) {
      logger.error("traeger_holiday_registrations_export_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: "die Exportdatei" });
    } finally {
      setExporting(null);
    }
  };

  const names = schoolNames(schools);
  const schoolOptions = [
    { value: "", label: "Alle Schulen" },
    ...(schools ?? []).map((entry) => ({ value: entry.id, label: entry.name })),
    { value: "guest", label: "Feriengäste" },
  ];
  const weekOptions = [
    { value: "", label: "Alle Wochen" },
    ...(period?.weeks ?? []).map((week) => ({
      value: week.id ?? "",
      label: `${formatHolidayWeek(week.startsOn, week.endsOn)}, ${names.get(week.hostSchoolId) ?? "Schule nicht mehr vorhanden"}`,
    })),
  ];
  const timeModelOptions = [
    { value: "", label: "Alle Zeitmodelle" },
    ...(period ? holidayTimeModelNames(period) : []).map((name) => ({
      value: name,
      label: name,
    })),
  ];
  const filtered = Boolean(status !== "all" || school || weekId || timeModel);
  const duplicates = duplicateCount(list);
  const loadError = listLoad.error ?? periodError ?? schoolsError;

  const filters: FilterConfig[] = [
    {
      id: "status",
      label: "Status",
      type: "dropdown",
      value: status,
      onChange: (value) => setStatus(value as StatusChoice),
      options: STATUS_OPTIONS,
    },
    {
      id: "school",
      label: "Schule",
      type: "dropdown",
      value: school,
      onChange: (value) => setSchool(value as string),
      options: schoolOptions,
    },
    {
      id: "week",
      label: "Woche",
      type: "dropdown",
      value: weekId,
      onChange: (value) => setWeekId(value as string),
      options: weekOptions,
    },
    {
      id: "timeModel",
      label: "Zeitmodell",
      type: "dropdown",
      value: timeModel,
      onChange: (value) => setTimeModel(value as string),
      options: timeModelOptions,
    },
  ];
  const optionLabel = (
    options: readonly { value: string; label: string }[],
    value: string,
  ) => options.find((option) => option.value === value)?.label ?? value;
  const activeFilters: ActiveFilter[] = [
    ...(status !== "all"
      ? [
          {
            id: "status",
            label: optionLabel(STATUS_OPTIONS, status),
            onRemove: () => setStatus("all"),
          },
        ]
      : []),
    ...(school
      ? [
          {
            id: "school",
            label: optionLabel(schoolOptions, school),
            onRemove: () => setSchool(""),
          },
        ]
      : []),
    ...(weekId
      ? [
          {
            id: "week",
            label: optionLabel(weekOptions, weekId),
            onRemove: () => setWeekId(""),
          },
        ]
      : []),
    ...(timeModel
      ? [
          {
            id: "timeModel",
            label: timeModel,
            onRemove: () => setTimeModel(""),
          },
        ]
      : []),
  ];
  const clearFilters = () => {
    setStatus("all");
    setSchool("");
    setWeekId("");
    setTimeModel("");
  };

  return (
    <TenantPage
      title={period?.name ?? preview?.name ?? "Ferienzeitraum"}
      back
      backHref={traegerPath("/traeger/ferien")}
      backLabel="Zurück zu den Ferien"
      stats={list ? statusLine(list) : null}
      statsLoading={!list && !loadError}
      tabs={holidayPeriodTabs(
        periodId,
        "anmeldungen",
        period?.registrationCount,
      )}
      actions={
        <ExportMenu
          ariaLabel="Anmeldungen herunterladen"
          options={EXPORT_FORMATS}
          busyKey={exporting}
          unavailable={loading || Boolean(loadError)}
          onSelect={(option) => {
            if (option.format === "csv" || option.format === "xlsx") {
              void exportFile(option.format);
            }
          }}
        />
      }
      filters={filters}
      activeFilters={activeFilters}
      onClearAllFilters={clearFilters}
      error={loadError}
    >
      {duplicates > 0 ? (
        <Alert
          type="warning"
          message={
            duplicates === 1
              ? "In einer Anmeldung ist ein Kind vielleicht doppelt angemeldet. Prüfen Sie die markierte Anmeldung."
              : `In ${duplicates} Anmeldungen ist ein Kind vielleicht doppelt angemeldet. Prüfen Sie die markierten Anmeldungen.`
          }
        />
      ) : null}
      <DataTable
        columns={columns(names)}
        rows={[...(list?.registrations ?? [])]}
        getRowKey={(registration) => registration.id}
        isLoading={loading && list === null}
        loadingRowCount={4}
        stackedOnMobile
        onRowClick={(registration) =>
          router.push(
            traegerPath(
              `/traeger/ferien/${periodId}/anmeldungen/${registration.id}`,
            ),
          )
        }
        emptyState={
          <EmptyState
            icon={<ClipboardList className="h-12 w-12" aria-hidden="true" />}
            title={
              filtered
                ? "Zu diesen Filtern gibt es keine Anmeldung."
                : "Es ist noch keine Anmeldung eingegangen."
            }
            description={
              filtered
                ? "Wählen Sie andere Filter."
                : "Eltern melden ihre Kinder über den Link für Eltern an."
            }
          />
        }
      />
    </TenantPage>
  );
}

"use client";

import {
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import Link from "next/link";
import {
  ArrowLeft,
  Check,
  ChevronDown,
  Clock,
  Download,
  ExternalLink,
  FileSpreadsheet,
  FileText,
  Inbox,
  Search,
  type LucideIcon,
  X,
} from "lucide-react";
import {
  type AdminRequestSummary,
  type ChildStatus,
  type DecisionStatus,
  decideAdminChild,
  listAdminRequests,
} from "~/lib/enrollment-admin-api";
import { listPhases, type Phase } from "~/lib/enrollment-phase-api";
import {
  type EnrollmentExportFormat,
  exportPhaseRegistrations,
} from "~/lib/enrollment-export-api";
import {
  type CareUsageFilters,
  type CareUsageReport,
  type CareUsageRow,
  type EnrollmentReportFormat,
  type EnrollmentReportLayout,
  type EnrollmentReportStatus,
  exportCareUsageReport,
  exportPhaseClassRoster,
  getCareUsageReport,
} from "~/lib/enrollment-report-api";
import {
  DataTable,
  type DataTableColumn,
  DataTableSkeleton,
  DataTableStatusBadge,
} from "~/components/ui/data-table";
import { SkeletonRegion } from "~/components/ui/page-skeletons";
import { Skeleton } from "~/components/ui/skeleton";
import { CustomSelect } from "~/components/ui/custom-select";
import { MultiCheckboxSelect } from "~/components/ui/multi-checkbox-select";
import { Alert } from "~/components/ui/alert";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  useCareOfferingsEnabled,
  useTenantSlugSafe,
} from "~/lib/tenant-context";
import { useToast } from "~/contexts/ToastContext";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { useClickOutside } from "~/lib/hooks/use-click-outside";
import { useEnrollmentPublicUrl } from "~/lib/enrollment-public-url";
import { PublicLinkCopyButton } from "~/components/enrollment/public-link-copy-button";
import { createLogger } from "~/lib/logger";
import { studentService } from "~/lib/api";
import { useSWRAuth } from "~/lib/swr";
import {
  CHILD_STATUS_LABELS,
  ChildStatusBadge,
} from "~/components/enrollment/child-status-badge";

const logger = createLogger({ component: "AdminEnrollmentPhaseDetail" });

const ALL_STATUS_FILTER = "all";
const ALL_VALUE = "all";
const DAY_COUNT_OPTIONS = [0, 1, 2, 3, 4, 5] as const;

const OPEN_STATUSES = new Set<ChildStatus>([
  "submitted",
  "under_review",
  "pending_admin_review",
]);

interface Props {
  readonly phaseId: string;
}

interface PhaseRequestStats {
  readonly total: number;
  readonly open: number;
  readonly approved: number;
  readonly rejected: number;
}

const TERMINAL_STATUSES = new Set<ChildStatus>([
  "approved",
  "rejected",
  "withdrawn",
]);

const EXPORT_MENU_LABELS: Record<EnrollmentExportFormat, string> = {
  pdf: "Als PDF exportieren",
  docx: "Als Word-Dokument exportieren",
  xlsx: "Als Excel-Datei exportieren",
};

const REPORT_EXPORT_LABELS: Record<EnrollmentReportFormat, string> = {
  docx: "Word",
  pdf: "PDF",
  xlsx: "Excel",
};

const DAY_LABELS: Record<string, string> = {
  mon: "Mo",
  tue: "Di",
  wed: "Mi",
  thu: "Do",
  fri: "Fr",
  sat: "Sa",
  sun: "So",
};

const CARE_USAGE_WEEKDAYS = ["mon", "tue", "wed", "thu", "fri"] as const;

export function AdminEnrollmentPhaseDetail({ phaseId }: Props) {
  const careOfferingsEnabled = useCareOfferingsEnabled();
  const tenantSlug = useTenantSlugSafe();
  const toast = useToast();
  const [phase, setPhase] = useState<Phase | null>(null);
  const [requests, setRequests] = useState<AdminRequestSummary[]>([]);
  const [statusFilter, setStatusFilter] =
    useState<EnrollmentReportStatus>(ALL_STATUS_FILTER);
  const [defaultOfferingIds, setDefaultOfferingIds] = useState<string[]>([]);
  const [explicitOfferingIds, setExplicitOfferingIds] = useState<
    string[] | null
  >(null);
  const [dayCount, setDayCount] = useState<string>(ALL_VALUE);
  const [gradeLevel, setGradeLevel] = useState<string>(ALL_VALUE);
  const [weekday, setWeekday] = useState<string>(ALL_VALUE);
  const [pickupTime, setPickupTime] = useState<string>(ALL_VALUE);
  const [classRosterSchoolClass, setClassRosterSchoolClass] =
    useState<string>(ALL_VALUE);
  const [search, setSearch] = useState("");
  const [report, setReport] = useState<CareUsageReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [reportLoading, setReportLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [reportError, setReportError] = useState<string | null>(null);
  const [busyChildId, setBusyChildId] = useState<string | null>(null);
  const [approvalWithoutOfferingRow, setApprovalWithoutOfferingRow] =
    useState<CareUsageRow | null>(null);
  const [exportingFormat, setExportingFormat] =
    useState<EnrollmentExportFormat | null>(null);
  const [exportingReportFormat, setExportingReportFormat] =
    useState<EnrollmentReportFormat | null>(null);
  const [exportingClassRosterFormat, setExportingClassRosterFormat] =
    useState<EnrollmentReportFormat | null>(null);
  const phaseUrl = useEnrollmentPublicUrl({ tenantSlug, phaseId });
  useSetBreadcrumb({ pageTitle: phase?.name ?? "Anmeldephase" });

  const { data: schoolClassOptionsData } = useSWRAuth<unknown>(
    "enrollment-phase-school-classes",
    async () => studentService.getSchoolClasses(),
    { revalidateOnFocus: false },
  );
  const schoolClassOptions = useMemo(
    () => coerceSchoolClassOptions(schoolClassOptionsData),
    [schoolClassOptionsData],
  );

  const handleExport = useCallback(
    async (format: EnrollmentExportFormat) => {
      setExportingFormat(format);
      try {
        // Export honours the active status filter so it matches the list
        // the admin is looking at; "Alle" means no filter.
        const childStatus =
          statusFilter === ALL_STATUS_FILTER ? undefined : statusFilter;
        await exportPhaseRegistrations(phaseId, format, childStatus);
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Export fehlgeschlagen";
        logger.error("phase_export_failed", { error: message, format });
        toast.error("Export fehlgeschlagen. Bitte erneut versuchen.");
      } finally {
        setExportingFormat(null);
      }
    },
    [phaseId, statusFilter, toast],
  );

  const reportFilters = useMemo<CareUsageFilters>(() => {
    const filters: CareUsageFilters = {
      phase_id: phaseId,
      status: statusFilter,
    };
    if (explicitOfferingIds !== null) {
      filters.care_offering_ids = explicitOfferingIds;
    }
    if (dayCount !== ALL_VALUE) {
      filters.day_count = Number(dayCount);
    }
    if (gradeLevel !== ALL_VALUE) {
      filters.grade_level = Number(gradeLevel);
    }
    if (weekday !== ALL_VALUE) {
      filters.weekday = weekday;
    }
    if (pickupTime !== ALL_VALUE) {
      filters.pickup_time = pickupTime;
    }
    const trimmedSearch = search.trim();
    if (trimmedSearch !== "") {
      filters.search = trimmedSearch;
    }
    return filters;
  }, [
    dayCount,
    explicitOfferingIds,
    gradeLevel,
    phaseId,
    pickupTime,
    search,
    statusFilter,
    weekday,
  ]);

  const handleReportOfferingsChange = useCallback((ids: string[]) => {
    setExplicitOfferingIds(ids);
  }, []);

  const loadReport = useCallback(
    async (filters: CareUsageFilters, isCancelled?: () => boolean) => {
      setReportLoading(true);
      setReportError(null);
      setReport(null);
      try {
        const data = await getCareUsageReport(filters);
        if (isCancelled?.()) return;
        setReport(data);
        setDefaultOfferingIds(
          data.filter_options.offerings
            .filter((offering) => offering.counts_as_care !== false)
            .map((offering) => offering.id),
        );
      } catch (err) {
        if (isCancelled?.()) return;
        const message =
          err instanceof Error
            ? err.message
            : "Auswertung konnte nicht geladen werden";
        logger.error("phase_care_usage_report_load_failed", {
          error: message,
          phase_id: phaseId,
        });
        setReportError(message);
      } finally {
        if (!isCancelled?.()) setReportLoading(false);
      }
    },
    [phaseId],
  );
  const displayedOfferingIds = explicitOfferingIds ?? defaultOfferingIds;

  useEffect(() => {
    let cancelled = false;
    void loadReport(reportFilters, () => cancelled);
    return () => {
      cancelled = true;
    };
  }, [loadReport, reportFilters]);

  const handleReportExport = useCallback(
    async (
      format: EnrollmentReportFormat,
      layout: EnrollmentReportLayout = "detailed",
    ) => {
      setExportingReportFormat(format);
      try {
        if (layout === "compact") {
          await exportCareUsageReport(reportFilters, format, "compact");
        } else {
          await exportCareUsageReport(reportFilters, format);
        }
        toast.success("Auswertungsexport wurde erstellt.");
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Export fehlgeschlagen";
        logger.error("phase_care_usage_report_export_failed", {
          error: message,
          format,
          layout,
          phase_id: phaseId,
        });
        toast.error(message);
      } finally {
        setExportingReportFormat(null);
      }
    },
    [phaseId, reportFilters, toast],
  );

  const handleClassRosterExport = useCallback(
    async (format: EnrollmentReportFormat) => {
      setExportingClassRosterFormat(format);
      try {
        await exportPhaseClassRoster(
          phaseId,
          classRosterSchoolClass === ALL_VALUE ? null : classRosterSchoolClass,
          format,
        );
        toast.success("Klassenliste wurde erstellt.");
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Export fehlgeschlagen";
        logger.error("phase_class_roster_export_failed", {
          error: message,
          format,
          phase_id: phaseId,
          school_class: classRosterSchoolClass,
        });
        toast.error(message);
      } finally {
        setExportingClassRosterFormat(null);
      }
    },
    [classRosterSchoolClass, phaseId, toast],
  );

  const overviewHref = tenantSlug
    ? `/${tenantSlug}/admin/enrollments`
    : "/admin/enrollments";

  const requestHref = useCallback(
    (requestId: string) =>
      tenantSlug
        ? `/${tenantSlug}/admin/enrollments/${requestId}`
        : `/admin/enrollments/${requestId}`,
    [tenantSlug],
  );

  const loadData = useCallback(
    async (isCancelled?: () => boolean) => {
      setLoading(true);
      setError(null);
      try {
        const [phasesData, requestsData] = await Promise.all([
          listPhases(),
          listAdminRequests({ phaseId }),
        ]);
        if (isCancelled?.()) return;
        setPhase(phasesData.find((item) => item.id === phaseId) ?? null);
        setRequests(requestsData);
      } catch (err) {
        if (isCancelled?.()) return;
        const message =
          err instanceof Error ? err.message : "Unbekannter Fehler";
        logger.error("admin_enrollment_phase_detail_load_failed", {
          error: message,
          phase_id: phaseId,
        });
        setError(message);
      } finally {
        if (!isCancelled?.()) setLoading(false);
      }
    },
    [phaseId],
  );

  useEffect(() => {
    let cancelled = false;
    void loadData(() => cancelled);
    return () => {
      cancelled = true;
    };
  }, [loadData]);

  const stats = useMemo(() => calculateRequestStats(requests), [requests]);

  const handleQuickDecision = useCallback(
    async (row: CareUsageRow, status: DecisionStatus) => {
      setBusyChildId(row.child_id);
      setError(null);
      try {
        await decideAdminChild(row.request_id, row.child_id, status);
        toast.success(
          `Entscheidung gespeichert: ${CHILD_STATUS_LABELS[status as ChildStatus]}`,
        );
        await loadData();
        await loadReport(reportFilters);
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unbekannter Fehler";
        logger.error("admin_enrollment_phase_quick_decision_failed", {
          error: message,
          request_id: row.request_id,
          child_id: row.child_id,
          status,
        });
        setError(message);
        toast.error(message);
      } finally {
        setBusyChildId(null);
      }
    },
    [loadData, loadReport, reportFilters, toast],
  );

  const requestQuickDecision = useCallback(
    (row: CareUsageRow, status: DecisionStatus) => {
      if (
        status === "approved" &&
        careOfferingsEnabled &&
        phase?.care_offering_selection_mode === "optional" &&
        row.offerings.length === 0
      ) {
        setApprovalWithoutOfferingRow(row);
        return;
      }
      void handleQuickDecision(row, status);
    },
    [
      careOfferingsEnabled,
      handleQuickDecision,
      phase?.care_offering_selection_mode,
    ],
  );

  const columns = useMemo<DataTableColumn<CareUsageRow>[]>(
    () => [
      {
        key: "child",
        header: "Kind",
        render: (row) => (
          <div>
            <p className="font-medium text-gray-900">
              {row.child_first_name} {row.child_last_name}
            </p>
            <p className="text-xs text-gray-500">
              {row.target_school_class
                ? row.target_school_class
                : row.target_grade_level
                  ? `${row.target_grade_level}. Klasse`
                  : "Keine Klassenstufe"}
            </p>
          </div>
        ),
        sortValue: (row) => `${row.child_last_name} ${row.child_first_name}`,
      },
      {
        key: "guardian",
        header: "Elternkontakt",
        render: (row) => (
          <div>
            <p className="font-medium text-gray-900">
              {row.guardian_first_name} {row.guardian_last_name}
            </p>
            <p className="text-xs text-gray-500">{row.guardian_email}</p>
          </div>
        ),
        sortValue: (row) =>
          `${row.guardian_last_name} ${row.guardian_first_name}`,
      },
      {
        key: "submitted",
        header: "Eingegangen",
        render: (row) => formatDateTime(row.submitted_at),
        sortValue: (row) => new Date(row.submitted_at).getTime(),
      },
      {
        key: "status",
        header: "Status",
        render: (row) => <ChildStatusBadge status={row.status} />,
        sortValue: (row) => CHILD_STATUS_LABELS[row.status],
      },
      {
        key: "offerings",
        header: "Betreuungsangebote",
        render: (row) => <OfferingList row={row} />,
        sortValue: (row) => row.offerings.map((item) => item.name).join(" "),
      },
      {
        key: "effective_days",
        header: "Betreuungstage",
        render: (row) => (
          <div>
            <p className="font-medium text-gray-900">
              {formatDays(row.effective_days) || "-"}
            </p>
            <p className="text-xs text-gray-500">
              {row.day_count === 1
                ? "1 Betreuungstag"
                : `${row.day_count} Betreuungstage`}
            </p>
          </div>
        ),
        sortValue: (row) => row.day_count,
      },
      {
        key: "pickup_by_day",
        header: "Gehzeiten",
        render: (row) => (
          <p className="max-w-48 text-sm text-gray-700">
            {formatPickupByDay(row.pickup_by_day ?? {}) || "-"}
          </p>
        ),
        sortValue: (row) => formatPickupByDay(row.pickup_by_day ?? {}),
      },
      {
        key: "actions",
        header: "Aktionen",
        align: "right",
        render: (row) => (
          <PhaseChildActions
            row={row}
            href={requestHref(row.request_id)}
            busy={busyChildId === row.child_id}
            onDecide={(status) => requestQuickDecision(row, status)}
          />
        ),
      },
    ],
    [busyChildId, requestHref, requestQuickDecision],
  );

  if (loading) {
    return (
      <SkeletonRegion label="Anmeldungen werden geladen">
        <div className="space-y-4">
          <div className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md sm:p-6">
            <Skeleton className="h-3 w-28 rounded" />
            <Skeleton className="mt-2 h-6 w-64 rounded" />
            <Skeleton className="mt-2 h-4 w-48 rounded" />
            <div className="mt-4 grid gap-3 md:grid-cols-4">
              {Array.from({ length: 4 }, (_, i) => (
                <Skeleton key={i} className="h-16 rounded-2xl" />
              ))}
            </div>
          </div>
          <DataTableSkeleton rows={8} columns={8} />
        </div>
      </SkeletonRegion>
    );
  }

  if (error) {
    return (
      <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-2xl border p-4 text-sm">
        {error}
      </div>
    );
  }

  if (!phase) {
    return (
      <section className="moto-content-surface rounded-2xl border p-6 shadow-sm backdrop-blur-md">
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          Anmeldephase
        </p>
        <h1 className="mt-1 text-xl font-semibold text-gray-900">
          Anmeldephase nicht gefunden
        </h1>
        <Link
          href={overviewHref}
          className="mt-4 inline-flex h-9 items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          Zurück zum Überblick
        </Link>
      </section>
    );
  }

  return (
    <div className="space-y-4">
      <section className="moto-content-surface rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="border-b border-gray-100 px-5 py-3 sm:px-6">
          <Link
            href={overviewHref}
            className="inline-flex h-8 items-center gap-2 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <ArrowLeft className="h-4 w-4" aria-hidden="true" />
            Zurück zum Überblick
          </Link>
        </div>
        <div className="flex flex-col gap-5 p-5 sm:p-6 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
              Anmeldephase
            </p>
            <div className="mt-1 flex flex-wrap items-center gap-2">
              <h1 className="text-xl font-semibold text-gray-900">
                {phase.name}
              </h1>
              <DataTableStatusBadge active={phase.is_active} />
            </div>
            <p className="mt-2 text-sm text-gray-600">
              {formatPhaseDate(phase.service_start_date)} bis{" "}
              {formatPhaseDate(phase.service_end_date)}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <ExportMenuButton
              label="Anmeldungen exportieren"
              menuAriaLabel="Exportformat auswählen"
              formats={["pdf", "docx", "xlsx"]}
              exportingFormat={exportingFormat}
              onExport={(format) => void handleExport(format)}
            />
            <a
              href={`/enroll/${encodeURIComponent(phase.id)}`}
              target="_blank"
              rel="noreferrer"
              className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              Elternansicht öffnen
              <ExternalLink className="h-4 w-4" aria-hidden="true" />
            </a>
            {phaseUrl ? (
              <PublicLinkCopyButton
                url={phaseUrl}
                componentId={`AdminEnrollmentPhaseDetail:${phaseId}`}
              />
            ) : null}
            <Link
              href="/enrollment-phases"
              className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              Phase bearbeiten
            </Link>
          </div>
        </div>

        <div className="grid gap-3 px-5 pb-5 sm:px-6 sm:pb-6 md:grid-cols-4">
          <StatCard icon={Inbox} label="Eingänge" value={stats.total} />
          <StatCard icon={Clock} label="Offen" value={stats.open} />
          <StatCard icon={Check} label="Bestätigt" value={stats.approved} />
          <StatCard icon={X} label="Abgelehnt" value={stats.rejected} />
        </div>
      </section>

      <section className="moto-content-surface relative z-20 overflow-visible rounded-2xl border p-4 shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-4">
          <div>
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              Eingänge
            </p>
            <h2 className="mt-1 text-base font-semibold text-gray-900">
              Anmeldungen prüfen und auswerten
            </h2>
            <p className="mt-1 text-sm text-gray-600">
              Filtere Kinder nach Status, Angebot, Zielklasse oder Anzahl der
              Betreuungstage. Der Auswertungsexport übernimmt die aktuellen
              Filter.
            </p>
          </div>
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-[repeat(6,minmax(9rem,1fr))]">
            <SelectField label="Status" id="enrollment-status-filter">
              <CustomSelect
                id="enrollment-status-filter"
                ariaLabel="Status"
                value={statusFilter}
                onChange={(value) =>
                  setStatusFilter(value as EnrollmentReportStatus)
                }
                options={[
                  { value: ALL_STATUS_FILTER, label: "Alle" },
                  ...Object.entries(CHILD_STATUS_LABELS).map(
                    ([value, label]) => ({
                      value,
                      label,
                    }),
                  ),
                ]}
              />
            </SelectField>
            <SelectField
              label="Berücksichtigte Angebote"
              id="enrollment-offering-filter"
            >
              <MultiCheckboxSelect
                id="enrollment-offering-filter"
                ariaLabel="Berücksichtigte Angebote"
                value={displayedOfferingIds}
                onChange={handleReportOfferingsChange}
                options={(report?.filter_options.offerings ?? []).map(
                  (offering) => ({
                    value: offering.id,
                    label: offering.name,
                    badge:
                      offering.counts_as_care === false
                        ? "Zählt nicht als Betreuungstag"
                        : undefined,
                  }),
                )}
                emptyLabel="Keine Angebote"
                unavailableLabel="Keine Angebote verfügbar"
                multipleLabel={(count) => `${count} Angebote`}
              />
            </SelectField>
            <SelectField
              label="Anzahl Betreuungstage"
              id="enrollment-day-count-filter"
            >
              <CustomSelect
                id="enrollment-day-count-filter"
                ariaLabel="Anzahl Betreuungstage"
                value={dayCount}
                onChange={setDayCount}
                options={[
                  { value: ALL_VALUE, label: "Alle" },
                  ...DAY_COUNT_OPTIONS.map((count) => ({
                    value: String(count),
                    label: formatDayCountLabel(count),
                  })),
                ]}
              />
            </SelectField>
            <SelectField label="Zielklasse" id="enrollment-grade-filter">
              <CustomSelect
                id="enrollment-grade-filter"
                ariaLabel="Zielklasse"
                value={gradeLevel}
                onChange={setGradeLevel}
                options={[
                  { value: ALL_VALUE, label: "Alle Klassen" },
                  ...(report?.filter_options.grade_levels ?? []).map(
                    (grade) => ({
                      value: String(grade),
                      label: `${grade}. Klasse`,
                    }),
                  ),
                ]}
              />
            </SelectField>
            <SelectField label="Wochentag" id="enrollment-weekday-filter">
              <CustomSelect
                id="enrollment-weekday-filter"
                ariaLabel="Wochentag"
                value={weekday}
                onChange={setWeekday}
                options={[
                  { value: ALL_VALUE, label: "Alle Tage" },
                  ...CARE_USAGE_WEEKDAYS.map((day) => ({
                    value: day,
                    label: DAY_LABELS[day] ?? day,
                  })),
                ]}
              />
            </SelectField>
            <SelectField label="Gehzeit" id="enrollment-pickup-time-filter">
              <CustomSelect
                id="enrollment-pickup-time-filter"
                ariaLabel="Gehzeit"
                value={pickupTime}
                onChange={setPickupTime}
                options={[
                  { value: ALL_VALUE, label: "Alle Gehzeiten" },
                  ...(report?.filter_options.pickup_times ?? []).map(
                    (time) => ({
                      value: time,
                      label: `${time} Uhr`,
                    }),
                  ),
                  ...(pickupTime !== ALL_VALUE &&
                  !(report?.filter_options.pickup_times ?? []).includes(
                    pickupTime,
                  )
                    ? [{ value: pickupTime, label: `${pickupTime} Uhr` }]
                    : []),
                ]}
              />
            </SelectField>
          </div>
          <label
            htmlFor="enrollment-search-filter"
            className="flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 shadow-sm focus-within:ring-2 focus-within:ring-gray-400"
          >
            <Search className="h-4 w-4 text-gray-400" aria-hidden="true" />
            <input
              id="enrollment-search-filter"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Nach Kind oder Elternkontakt suchen"
              className="h-10 min-w-0 flex-1 border-0 bg-transparent text-sm text-gray-900 outline-none placeholder:text-gray-400"
            />
          </label>
        </div>
      </section>

      <ClassRosterExportPanel
        schoolClassOptions={schoolClassOptions}
        selectedSchoolClass={classRosterSchoolClass}
        onSelectedSchoolClassChange={setClassRosterSchoolClass}
        exportingFormat={exportingClassRosterFormat}
        onExport={(format) => void handleClassRosterExport(format)}
      />

      {reportError ? (
        <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-2xl border p-4 text-sm">
          {reportError}
        </div>
      ) : null}

      <ReportStats
        report={report}
        loading={reportLoading}
        exportingFormat={exportingReportFormat}
        onExport={(format, layout) => void handleReportExport(format, layout)}
      />

      <DataTable
        columns={columns}
        rows={report?.rows ?? []}
        getRowKey={(row) => row.child_id}
        defaultSortKey="child"
        defaultSortDirection="asc"
        isLoading={reportLoading}
        emptyState={
          <div className="mx-auto max-w-md py-6">
            <p className="font-medium text-gray-900">
              {requests.length === 0 && !reportLoading
                ? "Noch keine Anmeldungen eingegangen"
                : "Keine Kinder für diese Filter gefunden"}
            </p>
            <p className="mt-1 text-sm text-gray-500">
              {requests.length === 0 && !reportLoading
                ? "Sobald Eltern das Formular absenden, erscheinen die Eingänge hier."
                : "Passe Status, Angebot, Betreuungstage, Zielklasse oder Suche an."}
            </p>
          </div>
        }
      />
      <ConfirmationModal
        isOpen={approvalWithoutOfferingRow !== null}
        onClose={() => setApprovalWithoutOfferingRow(null)}
        onConfirm={() => {
          const row = approvalWithoutOfferingRow;
          setApprovalWithoutOfferingRow(null);
          if (row !== null) void handleQuickDecision(row, "approved");
        }}
        title="Anmeldung bestätigen"
        confirmText="Trotzdem bestätigen"
      >
        <Alert
          type="warning"
          message="Für dieses Kind ist kein Betreuungsangebot gebucht. Das Kind wird trotzdem in die OGS aufgenommen."
        />
      </ConfirmationModal>
    </div>
  );
}

function SelectField({
  label,
  id,
  children,
}: Readonly<{
  label: string;
  id: string;
  children: ReactNode;
}>) {
  return (
    <label
      className="flex flex-col gap-1 text-sm font-medium text-gray-700"
      htmlFor={id}
    >
      {label}
      {children}
    </label>
  );
}

function ClassRosterExportPanel({
  schoolClassOptions,
  selectedSchoolClass,
  onSelectedSchoolClassChange,
  exportingFormat,
  onExport,
}: Readonly<{
  schoolClassOptions: string[];
  selectedSchoolClass: string;
  onSelectedSchoolClassChange: (value: string) => void;
  exportingFormat: EnrollmentReportFormat | null;
  onExport: (format: EnrollmentReportFormat) => void;
}>) {
  return (
    <section className="moto-content-surface relative z-10 rounded-2xl border p-4 shadow-sm backdrop-blur-md">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div className="min-w-0 md:w-72">
          <SelectField
            label="Klasse für Klassenliste"
            id="phase-class-roster-class"
          >
            <CustomSelect
              id="phase-class-roster-class"
              ariaLabel="Klasse für Klassenliste"
              value={selectedSchoolClass}
              onChange={onSelectedSchoolClassChange}
              options={[
                {
                  value: ALL_VALUE,
                  label:
                    schoolClassOptions.length === 0
                      ? "Keine Klassen"
                      : "Alle Klassen",
                },
                ...schoolClassOptions.map((schoolClass) => ({
                  value: schoolClass,
                  label: schoolClass,
                })),
              ]}
            />
          </SelectField>
        </div>

        <ExportMenuButton
          label="Klassenliste exportieren"
          menuAriaLabel="Klassenliste exportieren"
          formats={["pdf", "docx", "xlsx"]}
          exportingFormat={exportingFormat}
          disabled={schoolClassOptions.length === 0}
          onExport={onExport}
        />
      </div>
    </section>
  );
}

function ReportStats({
  report,
  loading,
  exportingFormat,
  onExport,
}: Readonly<{
  report: CareUsageReport | null;
  loading: boolean;
  exportingFormat: EnrollmentReportFormat | null;
  onExport: (
    format: EnrollmentReportFormat,
    layout: EnrollmentReportLayout,
  ) => void;
}>) {
  const totals = report?.totals;
  return (
    <section className="relative z-10 space-y-2">
      <div className="grid grid-cols-2 gap-2 md:grid-cols-4 xl:grid-cols-[repeat(7,minmax(0,1fr))_minmax(15rem,1.2fr)]">
        <ReportStatCard
          label="Kinder"
          value={loading ? "..." : String(totals?.children ?? 0)}
        />
        {DAY_COUNT_OPTIONS.map((count) => (
          <ReportStatCard
            key={count}
            label={formatDayCountLabel(count)}
            value={
              loading ? "..." : String(totals?.by_day_count[String(count)] ?? 0)
            }
          />
        ))}
        <ReportExportCard
          exportingFormat={exportingFormat}
          onExport={onExport}
        />
      </div>
      <DeploymentPlanningStats report={report} loading={loading} />
    </section>
  );
}

function DeploymentPlanningStats({
  report,
  loading,
}: Readonly<{
  report: CareUsageReport | null;
  loading: boolean;
}>) {
  const pickupTimes = useMemo(() => pickupTimesForReport(report), [report]);
  if (!loading && pickupTimes.length === 0) return null;

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          Einsatzplanung
        </p>
        <p className="text-sm font-medium text-gray-900">
          {loading ? "..." : `${report?.totals.children ?? 0} Kinder`}
        </p>
      </div>
      <div className="mt-3 grid gap-2 md:grid-cols-5">
        {CARE_USAGE_WEEKDAYS.map((day) => (
          <div
            key={day}
            className="moto-content-surface rounded-xl border px-3 py-2 shadow-sm backdrop-blur-md"
          >
            <p className="text-xs font-semibold text-gray-500">
              {DAY_LABELS[day]}
            </p>
            <div className="mt-2 flex flex-col gap-1">
              {loading ? (
                <p className="text-sm font-semibold text-gray-900">...</p>
              ) : (
                pickupTimes.map((time) => (
                  <div
                    key={`${day}-${time}`}
                    className="flex items-center justify-between gap-2 text-sm"
                  >
                    <span className="text-gray-600">{time}</span>
                    <span className="font-semibold text-gray-900">
                      {report?.totals.by_weekday_pickup_time?.[day]?.[time] ??
                        0}
                    </span>
                  </div>
                ))
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function ReportStatCard({
  label,
  value,
}: Readonly<{
  label: string;
  value: string;
}>) {
  return (
    <div className="moto-content-surface rounded-xl border px-3 py-2 shadow-sm backdrop-blur-md">
      <p className="truncate text-xs font-medium text-gray-500">{label}</p>
      <p className="mt-1 text-lg leading-none font-semibold text-gray-900">
        {value}
      </p>
    </div>
  );
}

// Excel is always tabular; PDF and Word offer the detailed record layout and
// the compact class-roster-style table (#2215).
const REPORT_EXPORT_OPTIONS: readonly ExportMenuItem[] = [
  { format: "xlsx", layout: "detailed", label: "Als Excel-Datei exportieren" },
  {
    format: "pdf",
    layout: "detailed",
    label: "Als PDF exportieren (ausführlich)",
  },
  {
    format: "pdf",
    layout: "compact",
    label: "Als PDF exportieren (kompakte Tabelle)",
  },
  {
    format: "docx",
    layout: "detailed",
    label: "Als Word-Dokument exportieren (ausführlich)",
  },
  {
    format: "docx",
    layout: "compact",
    label: "Als Word-Dokument exportieren (kompakte Tabelle)",
  },
];

function ReportExportCard({
  exportingFormat,
  onExport,
}: Readonly<{
  exportingFormat: EnrollmentReportFormat | null;
  onExport: (
    format: EnrollmentReportFormat,
    layout: EnrollmentReportLayout,
  ) => void;
}>) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const closeMenu = useCallback(() => setOpen(false), []);
  useClickOutside(containerRef, closeMenu, open);
  const disabled = exportingFormat !== null;

  return (
    <div
      className="moto-content-surface relative overflow-visible rounded-xl border px-3 py-2 shadow-sm backdrop-blur-md"
      ref={containerRef}
    >
      <p className="truncate text-xs font-medium text-gray-500">Auswertung</p>
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        disabled={disabled}
        aria-haspopup="menu"
        aria-expanded={open}
        className="mt-1 inline-flex min-h-6 w-full items-center justify-between gap-2 rounded-md text-left text-sm font-semibold text-gray-900 transition-colors hover:text-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60"
      >
        <span className="truncate">
          {exportingFormat === null
            ? "Auswertung exportieren"
            : `Exportiere ${REPORT_EXPORT_LABELS[exportingFormat]}...`}
        </span>
        <ChevronDown
          className={`h-4 w-4 shrink-0 text-gray-400 transition-transform ${
            open ? "rotate-180" : ""
          }`}
          aria-hidden="true"
        />
      </button>

      {open ? (
        <div
          role="menu"
          aria-label="Auswertung exportieren"
          className="moto-popover-surface absolute right-0 z-40 mt-2 min-w-64 overflow-hidden rounded-xl border py-1"
        >
          <ExportMenuItems
            items={REPORT_EXPORT_OPTIONS}
            onSelect={(item) => {
              setOpen(false);
              onExport(item.format, item.layout ?? "detailed");
            }}
          />
        </div>
      ) : null}
    </div>
  );
}

function OfferingList({ row }: Readonly<{ row: CareUsageRow }>) {
  if (row.offerings.length === 0) {
    return <span className="text-sm text-gray-500">Kein Angebot</span>;
  }
  return (
    <div className="space-y-1.5">
      {row.offerings.map((offering) => (
        <div key={offering.id} className="text-sm">
          <p className="font-medium text-gray-900">{offering.name}</p>
          <p className="text-xs text-gray-500">
            {formatOfferingDaySource(offering) || "Keine Tage"}
          </p>
        </div>
      ))}
    </div>
  );
}

function formatDays(days: string[]): string {
  return days.map((day) => DAY_LABELS[day] ?? day).join(", ");
}

function formatPickupByDay(pickupByDay: Record<string, string>): string {
  return CARE_USAGE_WEEKDAYS.map((day) => {
    const time = pickupByDay[day];
    return time ? `${DAY_LABELS[day]} ${time}` : "";
  })
    .filter(Boolean)
    .join("; ");
}

function pickupTimesForReport(report: CareUsageReport | null): string[] {
  const seen = new Set<string>(report?.filter_options.pickup_times ?? []);
  for (const byPickupTime of Object.values(
    report?.totals.by_weekday_pickup_time ?? {},
  )) {
    for (const pickupTime of Object.keys(byPickupTime)) {
      seen.add(pickupTime);
    }
  }
  return Array.from(seen).sort((a, b) => a.localeCompare(b));
}

function formatOfferingDaySource(
  offering: CareUsageRow["offerings"][number],
): string {
  if (offering.days_source !== "selected") {
    const days = formatDays(offering.days);
    return days ? `${days} · Angebotsumfang` : "";
  }
  const hasProvenance =
    (offering.manual_selected_days?.length ?? 0) > 0 ||
    (offering.automatic_selected_days?.length ?? 0) > 0;
  const automatic = formatDays(offering.automatic_selected_days ?? []);
  const manual = formatDays(
    hasProvenance ? (offering.manual_selected_days ?? []) : offering.days,
  );
  if (automatic && manual) {
    return `${automatic} automatisch mitgebucht; ${manual} von Eltern gewählt`;
  }
  if (automatic) return `${automatic} automatisch mitgebucht`;
  if (manual) return `${manual} · von Eltern gewählt`;
  return "";
}

function formatDayCountLabel(count: number): string {
  return count === 1 ? "1 Tag" : `${count} Tage`;
}

interface ExportMenuItem {
  readonly format: EnrollmentExportFormat;
  readonly label: string;
  readonly layout?: EnrollmentReportLayout;
}

function ExportMenuItems({
  items,
  onSelect,
}: {
  readonly items: readonly ExportMenuItem[];
  readonly onSelect: (item: ExportMenuItem) => void;
}) {
  return (
    <>
      {items.map((item) => (
        <button
          key={`${item.format}-${item.layout ?? "detailed"}`}
          type="button"
          role="menuitem"
          onClick={() => onSelect(item)}
          className="flex w-full items-center gap-2 px-4 py-2.5 text-left text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 active:bg-gray-100"
        >
          <ExportFormatIcon format={item.format} />
          <span className="flex-1">{item.label}</span>
        </button>
      ))}
    </>
  );
}

function ExportMenuButton({
  label,
  menuAriaLabel,
  formats,
  exportingFormat,
  disabled = false,
  onExport,
}: {
  readonly label: string;
  readonly menuAriaLabel: string;
  readonly formats: readonly EnrollmentExportFormat[];
  readonly exportingFormat: EnrollmentExportFormat | null;
  readonly disabled?: boolean;
  readonly onExport: (format: EnrollmentExportFormat) => void;
}) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const closeMenu = useCallback(() => setOpen(false), []);
  useClickOutside(containerRef, closeMenu, open);

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        disabled={disabled || exportingFormat !== null}
        aria-haspopup="menu"
        aria-expanded={open}
        className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-4 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60"
      >
        <Download className="h-4 w-4" aria-hidden="true" />
        {exportingFormat === null
          ? label
          : `Exportiere ${REPORT_EXPORT_LABELS[exportingFormat]}...`}
        <ChevronDown
          className={`h-4 w-4 transition-transform ${open ? "rotate-180" : ""}`}
          aria-hidden="true"
        />
      </button>

      {open ? (
        <div
          role="menu"
          aria-label={menuAriaLabel}
          className="moto-popover-surface absolute right-0 z-30 mt-2 min-w-64 overflow-hidden rounded-xl border py-1"
        >
          <ExportMenuItems
            items={formats.map((format) => ({
              format,
              label: EXPORT_MENU_LABELS[format],
            }))}
            onSelect={(item) => {
              setOpen(false);
              onExport(item.format);
            }}
          />
        </div>
      ) : null}
    </div>
  );
}

function ExportFormatIcon({
  format,
}: {
  readonly format: EnrollmentExportFormat;
}) {
  const Icon = format === "xlsx" ? FileSpreadsheet : FileText;
  return (
    <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-gray-100 text-gray-600">
      <Icon className="h-4 w-4" aria-hidden="true" />
    </span>
  );
}

function coerceSchoolClassOptions(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value
    .filter((item): item is string => typeof item === "string")
    .map((item) => item.trim())
    .filter(Boolean)
    .sort((a, b) =>
      a.localeCompare(b, "de", { numeric: true, sensitivity: "base" }),
    );
}

function calculateRequestStats(
  requests: AdminRequestSummary[],
): PhaseRequestStats {
  let total = 0;
  let open = 0;
  let approved = 0;
  let rejected = 0;

  for (const request of requests) {
    for (const child of request.children) {
      total += 1;
      if (OPEN_STATUSES.has(child.status)) open += 1;
      if (child.status === "approved") approved += 1;
      if (child.status === "rejected") rejected += 1;
    }
  }

  return { total, open, approved, rejected };
}

function formatPhaseDate(value: string): string {
  return new Date(`${value}T00:00:00`).toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

function formatDateTime(value: string): string {
  return new Date(value).toLocaleString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function PhaseChildActions({
  row,
  href,
  busy,
  onDecide,
}: Readonly<{
  row: CareUsageRow;
  href: string;
  busy: boolean;
  onDecide: (status: DecisionStatus) => void;
}>) {
  const terminal = TERMINAL_STATUSES.has(row.status);
  return (
    <div className="flex flex-wrap justify-end gap-2">
      {!terminal ? (
        <>
          <button
            type="button"
            disabled={busy}
            onClick={(event) => {
              event.stopPropagation();
              onDecide("approved");
            }}
            className="hover:border-moto-green/50 hover:bg-moto-green/10 hover:text-moto-green-strong inline-flex h-8 items-center justify-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 text-xs font-medium text-gray-700 shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Check className="text-moto-green h-3.5 w-3.5" aria-hidden="true" />
            {busy ? "Speichert..." : "Bestätigen"}
          </button>
          <button
            type="button"
            disabled={busy}
            onClick={(event) => {
              event.stopPropagation();
              onDecide("rejected");
            }}
            className="hover:border-moto-red/40 hover:bg-moto-red/10 hover:text-moto-red-strong inline-flex h-8 items-center justify-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 text-xs font-medium text-gray-700 shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          >
            <X className="text-moto-red h-3.5 w-3.5" aria-hidden="true" />
            Ablehnen
          </button>
        </>
      ) : null}
      <Link
        href={href}
        className="inline-flex h-8 items-center justify-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 text-xs font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        onClick={(event) => event.stopPropagation()}
      >
        Anmeldung ansehen
        <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
      </Link>
    </div>
  );
}

function StatCard({
  icon: Icon,
  label,
  value,
}: Readonly<{
  icon: LucideIcon;
  label: string;
  value: number;
}>) {
  return (
    <div className="moto-content-surface rounded-2xl border px-4 py-3 shadow-sm">
      <div className="flex items-center gap-3">
        <span className="flex h-9 w-9 items-center justify-center rounded-full bg-gray-50 text-gray-500 shadow-sm">
          <Icon className="h-4 w-4" aria-hidden="true" />
        </span>
        <span>
          <span className="block text-lg font-semibold text-gray-900">
            {value}
          </span>
          <span className="block text-xs font-medium text-gray-500">
            {label}
          </span>
        </span>
      </div>
    </div>
  );
}

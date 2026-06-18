"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import {
  BarChart3,
  Download,
  FileSpreadsheet,
  FileText,
  Filter,
  Search,
  type LucideIcon,
} from "lucide-react";
import { listPhases, type Phase } from "~/lib/enrollment-phase-api";
import {
  exportCareUsageReport,
  getCareUsageReport,
  type CareUsageFilters,
  type CareUsageReport,
  type CareUsageRow,
  type EnrollmentReportFormat,
  type EnrollmentReportStatus,
} from "~/lib/enrollment-report-api";
import type { ChildStatus } from "~/lib/enrollment-admin-api";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { CustomSelect } from "~/components/ui/custom-select";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentReportsPage" });

const STATUS_LABELS: Record<EnrollmentReportStatus, string> = {
  all: "Alle",
  submitted: "Eingegangen",
  under_review: "In Prüfung",
  approved: "Bestätigt",
  waitlisted: "Warteliste",
  rejected: "Abgelehnt",
  withdrawn: "Zurückgezogen",
  pending_renewal: "Wartet auf Verlängerung",
  auto_renewed: "Vorgemerkt",
  pending_admin_review: "Manuelle Prüfung",
};

const STATUS_OPTIONS: Array<{ value: EnrollmentReportStatus; label: string }> =
  [
    { value: "approved", label: STATUS_LABELS.approved },
    { value: "all", label: STATUS_LABELS.all },
    { value: "submitted", label: STATUS_LABELS.submitted },
    { value: "under_review", label: STATUS_LABELS.under_review },
    { value: "waitlisted", label: STATUS_LABELS.waitlisted },
    { value: "rejected", label: STATUS_LABELS.rejected },
    { value: "withdrawn", label: STATUS_LABELS.withdrawn },
    {
      value: "pending_admin_review",
      label: STATUS_LABELS.pending_admin_review,
    },
    { value: "pending_renewal", label: STATUS_LABELS.pending_renewal },
    { value: "auto_renewed", label: STATUS_LABELS.auto_renewed },
  ];

const DAY_LABELS: Record<string, string> = {
  mon: "Mo",
  tue: "Di",
  wed: "Mi",
  thu: "Do",
  fri: "Fr",
  sat: "Sa",
  sun: "So",
};

const ALL_VALUE = "all";

export function EnrollmentReportsPage() {
  const toast = useToast();
  const [phases, setPhases] = useState<Phase[]>([]);
  const [selectedPhaseId, setSelectedPhaseId] = useState<string>("");
  const [status, setStatus] = useState<EnrollmentReportStatus>("approved");
  const [offeringId, setOfferingId] = useState<string>(ALL_VALUE);
  const [dayCount, setDayCount] = useState<string>(ALL_VALUE);
  const [gradeLevel, setGradeLevel] = useState<string>(ALL_VALUE);
  const [search, setSearch] = useState("");
  const [report, setReport] = useState<CareUsageReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [reportLoading, setReportLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [exporting, setExporting] = useState<EnrollmentReportFormat | null>(
    null,
  );

  useEffect(() => {
    let cancelled = false;
    async function loadPhases() {
      setLoading(true);
      setError(null);
      try {
        const data = await listPhases();
        if (cancelled) return;
        const sorted = sortPhases(data);
        setPhases(sorted);
        setSelectedPhaseId((current) => current || sorted[0]?.id || "");
      } catch (err) {
        if (cancelled) return;
        const message =
          err instanceof Error
            ? err.message
            : "Phasen konnten nicht geladen werden";
        logger.error("enrollment_reports_phases_failed", { error: message });
        setError(message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void loadPhases();
    return () => {
      cancelled = true;
    };
  }, []);

  const filters = useMemo<CareUsageFilters | null>(() => {
    if (!selectedPhaseId) return null;
    return {
      phase_id: Number(selectedPhaseId),
      status,
      care_offering_id:
        offeringId === ALL_VALUE ? undefined : Number(offeringId),
      day_count: dayCount === ALL_VALUE ? undefined : Number(dayCount),
      grade_level: gradeLevel === ALL_VALUE ? undefined : Number(gradeLevel),
      search: search.trim() || undefined,
    };
  }, [dayCount, gradeLevel, offeringId, search, selectedPhaseId, status]);

  const loadReport = useCallback(
    async (activeFilters: CareUsageFilters, isCancelled?: () => boolean) => {
      setReportLoading(true);
      setError(null);
      try {
        const data = await getCareUsageReport(activeFilters);
        if (isCancelled?.()) return;
        setReport(data);
      } catch (err) {
        if (isCancelled?.()) return;
        const message =
          err instanceof Error
            ? err.message
            : "Auswertung konnte nicht geladen werden";
        logger.error("care_usage_report_load_failed", { error: message });
        setError(message);
      } finally {
        if (!isCancelled?.()) setReportLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    if (!filters) return;
    let cancelled = false;
    void loadReport(filters, () => cancelled);
    return () => {
      cancelled = true;
    };
  }, [filters, loadReport]);

  const handleExport = useCallback(
    async (format: EnrollmentReportFormat) => {
      if (!filters) return;
      setExporting(format);
      try {
        await exportCareUsageReport(filters, format);
        toast.success("Auswertung wurde exportiert.");
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Export fehlgeschlagen";
        logger.error("care_usage_report_export_failed", {
          error: message,
          format,
        });
        toast.error(message);
      } finally {
        setExporting(null);
      }
    },
    [filters, toast],
  );

  const handlePhaseChange = useCallback((value: string) => {
    setSelectedPhaseId(value);
    setOfferingId(ALL_VALUE);
    setDayCount(ALL_VALUE);
    setGradeLevel(ALL_VALUE);
  }, []);

  const columns = useMemo<DataTableColumn<CareUsageRow>[]>(
    () => [
      {
        key: "child",
        header: "Kind",
        render: (row) => (
          <div>
            <p className="font-semibold text-gray-900">
              {row.child_first_name} {row.child_last_name}
            </p>
            <p className="text-xs text-gray-500">
              {row.target_grade_level
                ? `${row.target_grade_level}. Klasse`
                : "Keine Zielklasse"}
            </p>
          </div>
        ),
        sortValue: (row) => `${row.child_last_name} ${row.child_first_name}`,
      },
      {
        key: "status",
        header: "Status",
        render: (row) => <StatusPill status={row.status} />,
        sortValue: (row) => STATUS_LABELS[row.status],
      },
      {
        key: "offerings",
        header: "Betreuungsangebot",
        render: (row) => <OfferingList row={row} />,
        sortValue: (row) => row.offerings.map((item) => item.name).join(" "),
      },
      {
        key: "effective_days",
        header: "Tage",
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
    ],
    [],
  );

  if (loading) {
    return <p className="text-sm text-gray-500">Auswertung wird geladen...</p>;
  }

  if (phases.length === 0) {
    return (
      <section className="moto-content-surface rounded-2xl border p-6 shadow-sm backdrop-blur-md">
        <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
          Auswertung
        </p>
        <h1 className="mt-1 text-xl font-semibold text-gray-900">
          Noch keine Anmeldephase
        </h1>
        <p className="mt-2 text-sm leading-6 text-gray-600">
          Lege zuerst eine Anmeldephase an. Danach kannst du hier
          Betreuungsangebote, Betreuungstage und Exporte auswerten.
        </p>
      </section>
    );
  }

  return (
    <div className="space-y-4">
      <section className="moto-content-surface rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-5 p-5 sm:p-6 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
              Auswertung
            </p>
            <h1 className="mt-1 text-xl font-semibold text-gray-900">
              Betreuungsangebot & Betreuungstage
            </h1>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
              Wähle eine Anmeldephase und filtere die Kinder nach Status,
              Angebot, Zielklasse oder Anzahl der effektiven Betreuungstage.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <ExportButton
              format="xlsx"
              label="Excel"
              exporting={exporting}
              icon={<FileSpreadsheet className="h-4 w-4" aria-hidden />}
              onExport={() => void handleExport("xlsx")}
            />
            <ExportButton
              format="pdf"
              label="PDF"
              exporting={exporting}
              icon={<FileText className="h-4 w-4" aria-hidden />}
              onExport={() => void handleExport("pdf")}
            />
          </div>
        </div>
      </section>

      <section className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur-md">
        <div className="grid gap-3 lg:grid-cols-[minmax(12rem,1.2fr)_repeat(4,minmax(9rem,1fr))]">
          <SelectField label="Anmeldephase" id="report-phase">
            <CustomSelect
              id="report-phase"
              value={selectedPhaseId}
              onChange={handlePhaseChange}
              options={phases.map((phase) => ({
                value: phase.id,
                label: phase.name,
              }))}
            />
          </SelectField>
          <SelectField label="Status" id="report-status">
            <CustomSelect
              id="report-status"
              value={status}
              onChange={(value) => setStatus(value as EnrollmentReportStatus)}
              options={STATUS_OPTIONS}
            />
          </SelectField>
          <SelectField label="Betreuungsangebot" id="report-offering">
            <CustomSelect
              id="report-offering"
              value={offeringId}
              onChange={setOfferingId}
              options={[
                { value: ALL_VALUE, label: "Alle Angebote" },
                ...(report?.filter_options.offerings ?? []).map((item) => ({
                  value: String(item.id),
                  label: item.name,
                })),
              ]}
            />
          </SelectField>
          <SelectField label="Tagesanzahl" id="report-day-count">
            <CustomSelect
              id="report-day-count"
              value={dayCount}
              onChange={setDayCount}
              options={[
                { value: ALL_VALUE, label: "Alle" },
                ...[1, 2, 3, 4, 5].map((count) => ({
                  value: String(count),
                  label: count === 1 ? "1 Tag" : `${count} Tage`,
                })),
              ]}
            />
          </SelectField>
          <SelectField label="Zielklasse" id="report-grade">
            <CustomSelect
              id="report-grade"
              value={gradeLevel}
              onChange={setGradeLevel}
              options={[
                { value: ALL_VALUE, label: "Alle Klassen" },
                ...(report?.filter_options.grade_levels ?? []).map((grade) => ({
                  value: String(grade),
                  label: `${grade}. Klasse`,
                })),
              ]}
            />
          </SelectField>
        </div>
        <label
          htmlFor="report-search"
          className="mt-3 flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 shadow-sm focus-within:ring-2 focus-within:ring-gray-400"
        >
          <Search className="h-4 w-4 text-gray-400" aria-hidden />
          <input
            id="report-search"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Nach Kind oder Elternkontakt suchen"
            className="h-10 min-w-0 flex-1 border-0 bg-transparent text-sm text-gray-900 outline-none placeholder:text-gray-400"
          />
        </label>
      </section>

      {error ? (
        <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
          {error}
        </div>
      ) : null}

      <ReportStats report={report} loading={reportLoading} />

      <DataTable
        columns={columns}
        rows={report?.rows ?? []}
        getRowKey={(row) => String(row.child_id)}
        defaultSortKey="child"
        defaultSortDirection="asc"
        emptyState={
          <div className="mx-auto max-w-md py-6">
            <p className="font-medium text-gray-900">
              {reportLoading
                ? "Auswertung wird geladen"
                : "Keine Kinder gefunden"}
            </p>
            <p className="mt-1 text-sm text-gray-500">
              {reportLoading
                ? "Die aktuellen Filter werden angewendet."
                : "Passe Status, Angebot, Zielklasse oder Suche an."}
            </p>
          </div>
        }
      />
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

function ReportStats({
  report,
  loading,
}: Readonly<{ report: CareUsageReport | null; loading: boolean }>) {
  const totals = report?.totals;
  return (
    <section className="grid gap-3 md:grid-cols-3 xl:grid-cols-6">
      <StatCard
        icon={BarChart3}
        label="Kinder"
        value={loading ? "..." : String(totals?.children ?? 0)}
      />
      {[1, 2, 3, 4, 5].map((count) => (
        <StatCard
          key={count}
          icon={Filter}
          label={count === 1 ? "1 Tag" : `${count} Tage`}
          value={
            loading ? "..." : String(totals?.by_day_count[String(count)] ?? 0)
          }
        />
      ))}
    </section>
  );
}

function StatCard({
  icon: Icon,
  label,
  value,
}: Readonly<{
  icon: LucideIcon;
  label: string;
  value: string;
}>) {
  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur-md">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-xs font-medium text-gray-500">{label}</p>
          <p className="mt-1 text-2xl font-semibold text-gray-900">{value}</p>
        </div>
        <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-[#5080D8]/10 text-[#5080D8]">
          <Icon className="h-5 w-5" aria-hidden />
        </span>
      </div>
    </div>
  );
}

function StatusPill({ status }: Readonly<{ status: ChildStatus }>) {
  return (
    <span className="inline-flex items-center rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-700">
      {STATUS_LABELS[status]}
    </span>
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
            {formatDays(offering.days) || "Keine Tage"}
            {offering.days_source === "selected"
              ? " · Elternauswahl"
              : " · Angebotsumfang"}
          </p>
        </div>
      ))}
    </div>
  );
}

function ExportButton({
  format,
  label,
  exporting,
  icon,
  onExport,
}: Readonly<{
  format: EnrollmentReportFormat;
  label: string;
  exporting: EnrollmentReportFormat | null;
  icon: ReactNode;
  onExport: () => void;
}>) {
  const disabled = exporting !== null;
  return (
    <button
      type="button"
      onClick={onExport}
      disabled={disabled}
      className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60"
    >
      {exporting === format ? (
        <Download className="h-4 w-4 animate-pulse" aria-hidden />
      ) : (
        icon
      )}
      {exporting === format ? "Exportiert..." : label}
    </button>
  );
}

function sortPhases(phases: Phase[]): Phase[] {
  return [...phases].sort((a, b) => {
    if (a.is_active !== b.is_active) return a.is_active ? -1 : 1;
    return b.service_start_date.localeCompare(a.service_start_date);
  });
}

function formatDays(days: string[]): string {
  return days.map((day) => DAY_LABELS[day] ?? day).join(", ");
}

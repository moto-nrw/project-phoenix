"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
  type LucideIcon,
  Users,
  X,
} from "lucide-react";
import {
  type AdminRequestChild,
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
  DataTable,
  type DataTableColumn,
  DataTableStatusBadge,
} from "~/components/ui/data-table";
import { useTenantSlugSafe } from "~/components/tenant/tenant-provider";
import { useToast } from "~/contexts/ToastContext";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { useClickOutside } from "~/lib/hooks/use-click-outside";
import { useEnrollmentPublicUrl } from "~/lib/enrollment-public-url";
import { PublicLinkCopyButton } from "~/components/enrollment/public-link-copy-button";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AdminEnrollmentPhaseDetail" });

const ALL_STATUS_FILTER = "all";

const STATUS_LABELS: Record<ChildStatus, string> = {
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

const STATUS_COLORS: Record<
  ChildStatus,
  { bg: string; dot: string; text: string }
> = {
  submitted: {
    bg: "#EEF3FF",
    dot: "#5080D8",
    text: "#355A9A",
  },
  under_review: {
    bg: "#EEF3FF",
    dot: "#5080D8",
    text: "#355A9A",
  },
  approved: {
    bg: "#83CD2D1A",
    dot: "#83CD2D",
    text: "#5A8B1F",
  },
  waitlisted: {
    bg: "#FFF4E6",
    dot: "#F78C10",
    text: "#8A5600",
  },
  rejected: {
    bg: "#FF31301A",
    dot: "#FF3130",
    text: "#9F1F1E",
  },
  withdrawn: {
    bg: "#F3F4F6",
    dot: "#9CA3AF",
    text: "#4B5563",
  },
  pending_renewal: {
    bg: "#FFF4E6",
    dot: "#F78C10",
    text: "#8A5600",
  },
  auto_renewed: {
    bg: "#EEF3FF",
    dot: "#5080D8",
    text: "#355A9A",
  },
  pending_admin_review: {
    bg: "#F3F4F6",
    dot: "#9CA3AF",
    text: "#4B5563",
  },
};

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

interface PhaseChildRow {
  readonly request: AdminRequestSummary;
  readonly child: AdminRequestChild;
}

const TERMINAL_STATUSES = new Set<ChildStatus>([
  "approved",
  "rejected",
  "withdrawn",
]);

const EXPORT_FORMAT_LABELS: Record<EnrollmentExportFormat, string> = {
  pdf: "Als PDF exportieren",
  docx: "Als Word-Dokument exportieren",
  xlsx: "Als Excel-Datei exportieren",
};

export function AdminEnrollmentPhaseDetail({ phaseId }: Props) {
  const tenantSlug = useTenantSlugSafe();
  const toast = useToast();
  const [phase, setPhase] = useState<Phase | null>(null);
  const [requests, setRequests] = useState<AdminRequestSummary[]>([]);
  const [statusFilter, setStatusFilter] = useState<
    ChildStatus | typeof ALL_STATUS_FILTER
  >(ALL_STATUS_FILTER);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyChildId, setBusyChildId] = useState<string | null>(null);
  const [exportingFormat, setExportingFormat] =
    useState<EnrollmentExportFormat | null>(null);
  const phaseUrl = useEnrollmentPublicUrl({ tenantSlug, phaseId });
  useSetBreadcrumb({ pageTitle: phase?.name ?? "Anmeldephase" });

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

  const childRows = useMemo(
    () =>
      requests.flatMap((request) =>
        request.children.map((child) => ({ request, child })),
      ),
    [requests],
  );

  const filteredChildRows = useMemo(() => {
    if (statusFilter === ALL_STATUS_FILTER) return childRows;
    return childRows.filter((row) => row.child.status === statusFilter);
  }, [childRows, statusFilter]);

  const handleQuickDecision = useCallback(
    async (row: PhaseChildRow, status: DecisionStatus) => {
      setBusyChildId(row.child.id);
      setError(null);
      try {
        await decideAdminChild(row.request.id, row.child.id, status);
        toast.success(
          `Entscheidung gespeichert: ${STATUS_LABELS[status as ChildStatus]}`,
        );
        await loadData();
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unbekannter Fehler";
        logger.error("admin_enrollment_phase_quick_decision_failed", {
          error: message,
          request_id: row.request.id,
          child_id: row.child.id,
          status,
        });
        setError(message);
        toast.error(message);
      } finally {
        setBusyChildId(null);
      }
    },
    [loadData, toast],
  );

  const columns = useMemo<DataTableColumn<PhaseChildRow>[]>(
    () => [
      {
        key: "guardian",
        header: "Eltern",
        render: (row) => (
          <div>
            <p className="font-semibold text-gray-900">
              {row.request.guardian_first_name} {row.request.guardian_last_name}
            </p>
            <p className="text-xs text-gray-500">
              {row.request.guardian_email}
            </p>
          </div>
        ),
        sortValue: (row) =>
          `${row.request.guardian_last_name} ${row.request.guardian_first_name}`,
      },
      {
        key: "submitted",
        header: "Eingegangen",
        render: (row) => formatDateTime(row.request.submitted_at),
        sortValue: (row) => new Date(row.request.submitted_at).getTime(),
      },
      {
        key: "child",
        header: "Kind",
        render: (row) => (
          <div>
            <p className="font-medium text-gray-900">
              {row.child.first_name} {row.child.last_name}
            </p>
            <p className="text-xs text-gray-500">
              {row.child.target_grade_level
                ? `${row.child.target_grade_level}. Klasse`
                : "Keine Klassenstufe"}
            </p>
          </div>
        ),
        sortValue: (row) => `${row.child.last_name} ${row.child.first_name}`,
      },
      {
        key: "status",
        header: "Status",
        render: (row) => <StatusBadge status={row.child.status} />,
        sortValue: (row) => STATUS_LABELS[row.child.status],
      },
      {
        key: "actions",
        header: "Aktionen",
        align: "right",
        render: (row) => (
          <PhaseChildActions
            row={row}
            href={requestHref(row.request.id)}
            busy={busyChildId === row.child.id}
            onDecide={(status) => void handleQuickDecision(row, status)}
          />
        ),
      },
    ],
    [busyChildId, handleQuickDecision, requestHref],
  );

  if (loading) {
    return (
      <p className="text-sm text-gray-500">Anmeldungen werden geladen...</p>
    );
  }

  if (error) {
    return (
      <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
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
            <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
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
            <ExportMenu
              exportingFormat={exportingFormat}
              onExport={(format) => void handleExport(format)}
            />
            <a
              href={`/enroll/${encodeURIComponent(phase.id)}`}
              target="_blank"
              rel="noreferrer"
              className="inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
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
          <StatCard icon={Users} label="Abgelehnt" value={stats.rejected} />
        </div>
      </section>

      <section className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              Eingänge
            </p>
            <h2 className="mt-1 text-base font-semibold text-gray-900">
              Anmeldungen prüfen
            </h2>
            <p className="mt-1 text-sm text-gray-600">
              Öffne eine Anmeldung, um Kinder anzunehmen, abzulehnen oder zur
              Prüfung zu markieren.
            </p>
          </div>
          <label className="flex flex-col gap-1 text-sm font-medium text-gray-700 sm:w-60">
            Status
            <select
              value={statusFilter}
              onChange={(event) =>
                setStatusFilter(
                  event.target.value as ChildStatus | typeof ALL_STATUS_FILTER,
                )
              }
              className="moto-select moto-content-surface h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              <option value={ALL_STATUS_FILTER}>Alle</option>
              {Object.entries(STATUS_LABELS).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </select>
          </label>
        </div>
      </section>

      <DataTable
        columns={columns}
        rows={filteredChildRows}
        getRowKey={(row) => row.child.id}
        defaultSortKey="submitted"
        defaultSortDirection="desc"
        emptyState={
          <div className="mx-auto max-w-md py-6">
            <p className="font-medium text-gray-900">
              {requests.length === 0
                ? "Noch keine Anmeldungen eingegangen"
                : "Keine Anmeldungen für diesen Status"}
            </p>
            <p className="mt-1 text-sm text-gray-500">
              {requests.length === 0
                ? "Sobald Eltern das Formular absenden, erscheinen die Eingänge hier."
                : "Wähle einen anderen Status, um weitere Anmeldungen zu sehen."}
            </p>
          </div>
        }
      />
    </div>
  );
}

function ExportMenu({
  exportingFormat,
  onExport,
}: {
  readonly exportingFormat: EnrollmentExportFormat | null;
  readonly onExport: (format: EnrollmentExportFormat) => void;
}) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  useClickOutside(containerRef, () => setOpen(false), open);

  const disabled = exportingFormat !== null;
  const formats: readonly EnrollmentExportFormat[] = ["pdf", "docx", "xlsx"];
  const triggerLabel =
    exportingFormat === null
      ? "Export"
      : `Exportiere ${exportingFormat.toUpperCase()}...`;

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        disabled={disabled}
        aria-haspopup="menu"
        aria-expanded={open}
        className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60"
      >
        <Download className="h-4 w-4" aria-hidden="true" />
        {triggerLabel}
        <ChevronDown
          className={`h-4 w-4 transition-transform ${open ? "rotate-180" : ""}`}
          aria-hidden="true"
        />
      </button>

      {open ? (
        <div
          role="menu"
          aria-label="Exportformat auswählen"
          className="absolute right-0 z-30 mt-2 min-w-64 overflow-hidden rounded-xl border border-gray-200 bg-white py-1 shadow-lg"
        >
          {formats.map((format) => (
            <button
              key={format}
              type="button"
              role="menuitem"
              onClick={() => {
                setOpen(false);
                onExport(format);
              }}
              className="flex w-full items-center gap-2 px-4 py-2.5 text-left text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 active:bg-gray-100"
            >
              <ExportFormatIcon format={format} />
              <span className="flex-1">{EXPORT_FORMAT_LABELS[format]}</span>
            </button>
          ))}
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
  row: PhaseChildRow;
  href: string;
  busy: boolean;
  onDecide: (status: DecisionStatus) => void;
}>) {
  const terminal = TERMINAL_STATUSES.has(row.child.status);
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
            className="inline-flex h-8 items-center justify-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 text-xs font-medium text-gray-700 shadow-sm transition-colors hover:border-[#83CD2D]/50 hover:bg-[#83CD2D]/10 hover:text-[#5A8B1F] focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Check className="h-3.5 w-3.5 text-[#83CD2D]" aria-hidden="true" />
            {busy ? "Speichert..." : "Bestätigen"}
          </button>
          <button
            type="button"
            disabled={busy}
            onClick={(event) => {
              event.stopPropagation();
              onDecide("rejected");
            }}
            className="inline-flex h-8 items-center justify-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 text-xs font-medium text-gray-700 shadow-sm transition-colors hover:border-[#FF3130]/40 hover:bg-[#FF3130]/10 hover:text-[#CC2626] focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          >
            <X className="h-3.5 w-3.5 text-[#FF3130]" aria-hidden="true" />
            Ablehnen
          </button>
        </>
      ) : null}
      <Link
        href={href}
        className="inline-flex h-8 items-center justify-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 text-xs font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        onClick={(event) => event.stopPropagation()}
      >
        Öffnen
        <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
      </Link>
    </div>
  );
}

function StatusBadge({ status }: Readonly<{ status: ChildStatus }>) {
  const styles = STATUS_COLORS[status];
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
      style={{ backgroundColor: styles.bg, color: styles.text }}
    >
      <span
        className="h-1.5 w-1.5 rounded-full"
        style={{ backgroundColor: styles.dot }}
        aria-hidden="true"
      />
      {STATUS_LABELS[status]}
    </span>
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
    <div className="rounded-2xl border border-gray-200 bg-white px-4 py-3 shadow-sm">
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

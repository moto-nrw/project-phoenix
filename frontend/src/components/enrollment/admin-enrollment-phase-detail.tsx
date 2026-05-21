"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  Check,
  Clock,
  ExternalLink,
  Inbox,
  type LucideIcon,
  Users,
} from "lucide-react";
import {
  type AdminRequestSummary,
  type ChildStatus,
  listAdminRequests,
} from "~/lib/enrollment-admin-api";
import { listPhases, type Phase } from "~/lib/enrollment-phase-api";
import {
  DataTable,
  type DataTableColumn,
  DataTableStatusBadge,
} from "~/components/ui/data-table";
import { useTenantSlugSafe } from "~/components/tenant/tenant-provider";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { LOCATION_COLORS } from "~/lib/location-helper";
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

const STATUS_COLORS: Record<ChildStatus, { bg: string; text: string }> = {
  submitted: { bg: LOCATION_COLORS.OTHER_ROOM, text: "#FFFFFF" },
  under_review: { bg: LOCATION_COLORS.OTHER_ROOM, text: "#FFFFFF" },
  approved: { bg: LOCATION_COLORS.GROUP_ROOM, text: "#FFFFFF" },
  waitlisted: { bg: LOCATION_COLORS.SCHOOLYARD, text: "#FFFFFF" },
  rejected: { bg: LOCATION_COLORS.HOME, text: "#FFFFFF" },
  withdrawn: { bg: LOCATION_COLORS.UNKNOWN, text: "#FFFFFF" },
  pending_renewal: { bg: LOCATION_COLORS.SCHOOLYARD, text: "#FFFFFF" },
  auto_renewed: { bg: LOCATION_COLORS.OTHER_ROOM, text: "#FFFFFF" },
  pending_admin_review: { bg: LOCATION_COLORS.UNKNOWN, text: "#FFFFFF" },
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

export function AdminEnrollmentPhaseDetail({ phaseId }: Props) {
  const tenantSlug = useTenantSlugSafe();
  const [phase, setPhase] = useState<Phase | null>(null);
  const [requests, setRequests] = useState<AdminRequestSummary[]>([]);
  const [statusFilter, setStatusFilter] = useState<
    ChildStatus | typeof ALL_STATUS_FILTER
  >(ALL_STATUS_FILTER);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  useSetBreadcrumb({ pageTitle: phase?.name ?? "Anmeldephase" });

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

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const [phasesData, requestsData] = await Promise.all([
          listPhases(),
          listAdminRequests({ phaseId }),
        ]);
        if (cancelled) return;
        setPhase(phasesData.find((item) => item.id === phaseId) ?? null);
        setRequests(requestsData);
      } catch (err) {
        if (cancelled) return;
        const message =
          err instanceof Error ? err.message : "Unbekannter Fehler";
        logger.error("admin_enrollment_phase_detail_load_failed", {
          error: message,
          phase_id: phaseId,
        });
        setError(message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [phaseId]);

  const stats = useMemo(() => calculateRequestStats(requests), [requests]);

  const filteredRequests = useMemo(() => {
    if (statusFilter === ALL_STATUS_FILTER) return requests;
    return requests.filter((request) =>
      request.children.some((child) => child.status === statusFilter),
    );
  }, [requests, statusFilter]);

  const columns = useMemo<DataTableColumn<AdminRequestSummary>[]>(
    () => [
      {
        key: "guardian",
        header: "Eltern",
        render: (request) => (
          <div>
            <p className="font-semibold text-gray-900">
              {request.guardian_first_name} {request.guardian_last_name}
            </p>
            <p className="text-xs text-gray-500">{request.guardian_email}</p>
          </div>
        ),
        sortValue: (request) =>
          `${request.guardian_last_name} ${request.guardian_first_name}`,
      },
      {
        key: "submitted",
        header: "Eingegangen",
        render: (request) => formatDateTime(request.submitted_at),
        sortValue: (request) => new Date(request.submitted_at).getTime(),
      },
      {
        key: "children",
        header: "Kinder",
        render: (request) => (
          <div className="space-y-1">
            {request.children.map((child) => (
              <div key={child.id} className="flex items-center gap-2">
                <span className="text-gray-900">
                  {child.first_name} {child.last_name}
                </span>
                <StatusBadge status={child.status} />
              </div>
            ))}
          </div>
        ),
      },
      {
        key: "actions",
        header: "Aktionen",
        align: "right",
        render: (request) => (
          <Link
            href={requestHref(request.id)}
            className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            onClick={(event) => event.stopPropagation()}
          >
            Öffnen
            <ExternalLink className="h-4 w-4" aria-hidden="true" />
          </Link>
        ),
      },
    ],
    [requestHref],
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
      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
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
          <Link
            href="/enrollment-phases"
            className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            Phase bearbeiten
          </Link>
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
        rows={filteredRequests}
        getRowKey={(request) => request.id}
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

function StatusBadge({ status }: Readonly<{ status: ChildStatus }>) {
  const styles = STATUS_COLORS[status];
  return (
    <span
      className="inline-flex rounded-full px-2 py-0.5 text-[10px] font-medium"
      style={{ backgroundColor: styles.bg, color: styles.text }}
    >
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

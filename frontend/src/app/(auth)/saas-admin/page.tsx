"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  type Organization,
  approveOrganization,
  fetchOrganizations,
  reactivateOrganization,
  rejectOrganization,
  suspendOrganization,
} from "~/lib/admin-api";
import { useSession } from "~/lib/auth-client";
import { Loading } from "~/components/ui/loading";
import { cn } from "~/lib/utils";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { Skeleton } from "~/components/ui/skeleton";

const STATUS_LABELS: Record<Organization["status"], string> = {
  pending: "Ausstehend",
  active: "Aktiv",
  rejected: "Abgelehnt",
  suspended: "Gesperrt",
};

const STATUS_BADGE_VARIANT: Record<
  Organization["status"],
  "default" | "success" | "warning" | "danger" | "secondary"
> = {
  pending: "warning",
  active: "success",
  rejected: "danger",
  suspended: "secondary",
};

type StatusFilter = Organization["status"] | "all";

// Stats card component
function StatsCard({
  title,
  value,
  description,
  icon,
  variant = "default",
}: {
  title: string;
  value: number;
  description?: string;
  icon: React.ReactNode;
  variant?: "default" | "warning" | "success" | "danger" | "secondary";
}) {
  const variantStyles = {
    default: "bg-gray-50 text-gray-600",
    warning: "bg-yellow-50 text-yellow-600",
    success: "bg-green-50 text-green-600",
    danger: "bg-red-50 text-red-600",
    secondary: "bg-gray-100 text-gray-500",
  };

  return (
    <Card className="transition-shadow hover:shadow-md">
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium text-gray-500">
          {title}
        </CardTitle>
        <div className={cn("rounded-lg p-2", variantStyles[variant])}>
          {icon}
        </div>
      </CardHeader>
      <CardContent>
        <div className="text-3xl font-bold text-gray-900 tabular-nums">
          {value}
        </div>
        {description && (
          <p className="mt-1 text-xs text-gray-500">{description}</p>
        )}
      </CardContent>
    </Card>
  );
}

// Icon components
function ClockIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn("size-5", className)}
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"
      />
    </svg>
  );
}

function CheckCircleIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn("size-5", className)}
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"
      />
    </svg>
  );
}

function XCircleIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn("size-5", className)}
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="m9.75 9.75 4.5 4.5m0-4.5-4.5 4.5M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"
      />
    </svg>
  );
}

function PauseCircleIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn("size-5", className)}
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M14.25 9v6m-4.5 0V9M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"
      />
    </svg>
  );
}

function BuildingIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn("size-5", className)}
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M2.25 21h19.5m-18-18v18m10.5-18v18m6-13.5V21M6.75 6.75h.75m-.75 3h.75m-.75 3h.75m3-6h.75m-.75 3h.75m-.75 3h.75M6.75 21v-3.375c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21M3 3h12m-.75 4.5H21m-3.75 3.75h.008v.008h-.008v-.008Zm0 3h.008v.008h-.008v-.008Zm0 3h.008v.008h-.008v-.008Z"
      />
    </svg>
  );
}

// Loading skeleton for stats
function StatsLoadingSkeleton() {
  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
      {Array.from({ length: 4 }).map((_, i) => (
        <Card key={i}>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="size-9 rounded-lg" />
          </CardHeader>
          <CardContent>
            <Skeleton className="h-8 w-16" />
            <Skeleton className="mt-2 h-3 w-32" />
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

// Table loading skeleton
function TableLoadingSkeleton() {
  return (
    <Card>
      <CardContent className="p-0">
        <div className="divide-y divide-gray-100">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="flex items-center gap-4 p-4">
              <Skeleton className="size-10 rounded-full" />
              <div className="flex-1 space-y-2">
                <Skeleton className="h-4 w-48" />
                <Skeleton className="h-3 w-32" />
              </div>
              <Skeleton className="h-6 w-20 rounded-full" />
              <Skeleton className="h-8 w-24 rounded-lg" />
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

// Empty state component
function EmptyState({
  filter,
  onResetFilter,
}: {
  filter: StatusFilter;
  onResetFilter: () => void;
}) {
  const messages: Record<StatusFilter, { title: string; description: string }> =
    {
      pending: {
        title: "Keine ausstehenden Anfragen",
        description:
          "Es gibt derzeit keine Organisationen, die auf Genehmigung warten.",
      },
      active: {
        title: "Keine aktiven Organisationen",
        description: "Es gibt derzeit keine aktiven Organisationen.",
      },
      rejected: {
        title: "Keine abgelehnten Anfragen",
        description: "Es gibt derzeit keine abgelehnten Organisationen.",
      },
      suspended: {
        title: "Keine gesperrten Organisationen",
        description: "Es gibt derzeit keine gesperrten Organisationen.",
      },
      all: {
        title: "Keine Organisationen",
        description:
          "Es wurden noch keine Organisationen registriert. Neue Registrierungen werden hier angezeigt.",
      },
    };

  const { title, description } = messages[filter];

  return (
    <Card className="border-dashed">
      <CardContent className="flex flex-col items-center justify-center py-16">
        <div className="flex size-16 items-center justify-center rounded-full bg-gray-100">
          <BuildingIcon className="size-8 text-gray-400" />
        </div>
        <h3 className="mt-6 text-lg font-semibold text-gray-900">{title}</h3>
        <p className="mt-2 max-w-sm text-center text-sm text-gray-500">
          {description}
        </p>
        {filter !== "all" && (
          <button
            onClick={onResetFilter}
            className="mt-6 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800"
          >
            Alle Organisationen anzeigen
          </button>
        )}
      </CardContent>
    </Card>
  );
}

// Filter tabs component
function FilterTabs({
  activeFilter,
  onFilterChange,
  counts,
}: {
  activeFilter: StatusFilter;
  onFilterChange: (filter: StatusFilter) => void;
  counts: Record<StatusFilter, number>;
}) {
  const filters: { value: StatusFilter; label: string }[] = [
    { value: "pending", label: "Ausstehend" },
    { value: "active", label: "Aktiv" },
    { value: "suspended", label: "Gesperrt" },
    { value: "rejected", label: "Abgelehnt" },
    { value: "all", label: "Alle" },
  ];

  return (
    <div className="inline-flex rounded-lg border border-gray-200 bg-gray-50 p-1">
      {filters.map((filter) => (
        <button
          key={filter.value}
          onClick={() => onFilterChange(filter.value)}
          className={cn(
            "relative rounded-md px-4 py-2 text-sm font-medium transition-all",
            activeFilter === filter.value
              ? "bg-white text-gray-900 shadow-sm"
              : "text-gray-600 hover:text-gray-900",
          )}
        >
          {filter.label}
          {counts[filter.value] > 0 && filter.value !== "all" && (
            <span
              className={cn(
                "ml-2 inline-flex items-center justify-center rounded-full px-2 py-0.5 text-xs font-medium tabular-nums",
                activeFilter === filter.value
                  ? "bg-gray-900 text-white"
                  : "bg-gray-200 text-gray-700",
              )}
            >
              {counts[filter.value]}
            </span>
          )}
        </button>
      ))}
    </div>
  );
}

// Organization row component
function OrganizationRow({
  org,
  actionLoading,
  onApprove,
  onReject,
  onSuspend,
  onReactivate,
}: {
  org: Organization;
  actionLoading: string | null;
  onApprove: (id: string) => void;
  onReject: (id: string) => void;
  onSuspend: (id: string) => void;
  onReactivate: (id: string) => void;
}) {
  const isLoading = actionLoading === org.id;

  return (
    <tr className="group transition-colors hover:bg-gray-50">
      <td className="px-6 py-4">
        <div className="flex items-center gap-4">
          <div className="flex size-10 flex-shrink-0 items-center justify-center rounded-full bg-gray-100 text-sm font-semibold text-gray-600">
            {org.name.charAt(0).toUpperCase()}
          </div>
          <div>
            <div className="font-medium text-gray-900">{org.name}</div>
            <div className="font-mono text-xs text-gray-500">{org.slug}</div>
          </div>
        </div>
      </td>
      <td className="px-6 py-4">
        <div className="text-sm text-gray-900">{org.ownerName ?? "-"}</div>
        <div className="text-sm text-gray-500">{org.ownerEmail ?? "-"}</div>
      </td>
      <td className="px-6 py-4">
        <Badge variant={STATUS_BADGE_VARIANT[org.status]}>
          {STATUS_LABELS[org.status]}
        </Badge>
      </td>
      <td className="px-6 py-4 text-sm text-gray-500 tabular-nums">
        {new Date(org.createdAt).toLocaleDateString("de-DE", {
          day: "2-digit",
          month: "2-digit",
          year: "numeric",
        })}
      </td>
      <td className="px-6 py-4 text-right">
        <div className="flex justify-end gap-2">
          {org.status === "pending" && (
            <>
              <button
                onClick={() => onApprove(org.id)}
                disabled={isLoading}
                className="inline-flex items-center gap-1.5 rounded-lg bg-green-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-green-700 disabled:opacity-50"
              >
                <CheckCircleIcon className="size-4" />
                {isLoading ? "..." : "Genehmigen"}
              </button>
              <button
                onClick={() => onReject(org.id)}
                disabled={isLoading}
                className="inline-flex items-center gap-1.5 rounded-lg bg-red-50 px-3 py-1.5 text-sm font-medium text-red-700 ring-1 ring-red-200 transition-colors hover:bg-red-100 disabled:opacity-50"
              >
                <XCircleIcon className="size-4" />
                Ablehnen
              </button>
            </>
          )}
          {org.status === "active" && (
            <button
              onClick={() => onSuspend(org.id)}
              disabled={isLoading}
              className="inline-flex items-center gap-1.5 rounded-lg bg-gray-100 px-3 py-1.5 text-sm font-medium text-gray-700 ring-1 ring-gray-200 transition-colors hover:bg-gray-200 disabled:opacity-50"
            >
              <PauseCircleIcon className="size-4" />
              {isLoading ? "..." : "Sperren"}
            </button>
          )}
          {org.status === "suspended" && (
            <button
              onClick={() => onReactivate(org.id)}
              disabled={isLoading}
              className="inline-flex items-center gap-1.5 rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:opacity-50"
            >
              <CheckCircleIcon className="size-4" />
              {isLoading ? "..." : "Reaktivieren"}
            </button>
          )}
        </div>
      </td>
    </tr>
  );
}

// Reject modal component
function RejectModal({
  isOpen,
  onClose,
  onConfirm,
  isLoading,
}: {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: (reason: string) => void;
  isLoading: boolean;
}) {
  const [reason, setReason] = useState("");

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div
        className="absolute inset-0 bg-gray-900/50 backdrop-blur-sm"
        onClick={onClose}
      />
      <Card className="relative z-10 w-full max-w-md shadow-xl">
        <CardHeader>
          <CardTitle>Organisation ablehnen</CardTitle>
          <CardDescription>
            Der Inhaber wird per E-Mail über die Ablehnung benachrichtigt.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <label className="mb-2 block text-sm font-medium text-gray-700">
            Ablehnungsgrund (optional)
          </label>
          <textarea
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Geben Sie einen Grund für die Ablehnung an..."
            className="w-full rounded-lg border border-gray-200 bg-white p-3 text-sm placeholder:text-gray-400 focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none"
            rows={4}
          />
        </CardContent>
        <div className="flex justify-end gap-3 border-t border-gray-100 p-6">
          <button
            onClick={onClose}
            className="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100"
          >
            Abbrechen
          </button>
          <button
            onClick={() => onConfirm(reason)}
            disabled={isLoading}
            className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:opacity-50"
          >
            {isLoading ? "Wird abgelehnt..." : "Ablehnen"}
          </button>
        </div>
      </Card>
    </div>
  );
}

export default function SaasAdminPage() {
  const { data: session, isPending: isSessionLoading } = useSession();
  const [allOrganizations, setAllOrganizations] = useState<Organization[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("pending");
  const [showRejectModal, setShowRejectModal] = useState<string | null>(null);

  // Calculate stats from all organizations
  const stats = useMemo(() => {
    const counts: Record<StatusFilter, number> = {
      pending: 0,
      active: 0,
      rejected: 0,
      suspended: 0,
      all: allOrganizations.length,
    };

    for (const org of allOrganizations) {
      counts[org.status]++;
    }

    return counts;
  }, [allOrganizations]);

  // Filter organizations based on status
  const filteredOrganizations = useMemo(() => {
    if (statusFilter === "all") return allOrganizations;
    return allOrganizations.filter((org) => org.status === statusFilter);
  }, [allOrganizations, statusFilter]);

  const loadOrganizations = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      // Fetch all organizations to calculate stats
      const orgs = await fetchOrganizations();
      setAllOrganizations(orgs);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Fehler beim Laden");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!isSessionLoading && session?.user) {
      void loadOrganizations();
    }
  }, [isSessionLoading, session, loadOrganizations]);

  const handleApprove = async (orgId: string) => {
    try {
      setActionLoading(orgId);
      await approveOrganization(orgId);
      await loadOrganizations();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Fehler beim Genehmigen");
    } finally {
      setActionLoading(null);
    }
  };

  const handleReject = async (orgId: string, reason: string) => {
    try {
      setActionLoading(orgId);
      await rejectOrganization(orgId, reason || undefined);
      setShowRejectModal(null);
      await loadOrganizations();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Fehler beim Ablehnen");
    } finally {
      setActionLoading(null);
    }
  };

  const handleSuspend = async (orgId: string) => {
    try {
      setActionLoading(orgId);
      await suspendOrganization(orgId);
      await loadOrganizations();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Fehler beim Sperren");
    } finally {
      setActionLoading(null);
    }
  };

  const handleReactivate = async (orgId: string) => {
    try {
      setActionLoading(orgId);
      await reactivateOrganization(orgId);
      await loadOrganizations();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Fehler beim Reaktivieren");
    } finally {
      setActionLoading(null);
    }
  };

  if (isSessionLoading) {
    return <Loading fullPage={false} />;
  }

  if (!session?.user) {
    return (
      <div className="flex min-h-dvh items-center justify-center p-4">
        <Card className="max-w-md">
          <CardContent className="flex flex-col items-center py-12 text-center">
            <div className="flex size-16 items-center justify-center rounded-full bg-red-100">
              <XCircleIcon className="size-8 text-red-600" />
            </div>
            <h1 className="mt-6 text-xl font-bold text-gray-900">
              Nicht autorisiert
            </h1>
            <p className="mt-2 text-gray-600">
              Du musst angemeldet sein, um diese Seite zu sehen.
            </p>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="min-h-dvh bg-gray-50/50">
      <div className="container mx-auto max-w-7xl px-4 py-8">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight text-balance text-gray-900">
            Organisation-Verwaltung
          </h1>
          <p className="mt-2 text-pretty text-gray-600">
            Verwalte Organisation-Registrierungen und deren Status
          </p>
        </div>

        {/* Stats Cards */}
        {loading ? (
          <StatsLoadingSkeleton />
        ) : (
          <div className="mb-8 grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            <StatsCard
              title="Ausstehend"
              value={stats.pending}
              description="Warten auf Genehmigung"
              icon={<ClockIcon />}
              variant="warning"
            />
            <StatsCard
              title="Aktiv"
              value={stats.active}
              description="Genehmigte Organisationen"
              icon={<CheckCircleIcon />}
              variant="success"
            />
            <StatsCard
              title="Gesperrt"
              value={stats.suspended}
              description="Temporär deaktiviert"
              icon={<PauseCircleIcon />}
              variant="secondary"
            />
            <StatsCard
              title="Abgelehnt"
              value={stats.rejected}
              description="Nicht genehmigt"
              icon={<XCircleIcon />}
              variant="danger"
            />
          </div>
        )}

        {/* Error Message */}
        {error && (
          <div className="mb-6 flex items-center justify-between rounded-lg border border-red-200 bg-red-50 p-4">
            <div className="flex items-center gap-3">
              <XCircleIcon className="size-5 text-red-600" />
              <span className="text-sm font-medium text-red-800">{error}</span>
            </div>
            <button
              onClick={() => setError(null)}
              className="text-sm font-medium text-red-600 hover:text-red-800"
            >
              Schließen
            </button>
          </div>
        )}

        {/* Filter Tabs */}
        <div className="mb-6">
          <FilterTabs
            activeFilter={statusFilter}
            onFilterChange={setStatusFilter}
            counts={stats}
          />
        </div>

        {/* Organizations Table */}
        {loading ? (
          <TableLoadingSkeleton />
        ) : filteredOrganizations.length === 0 ? (
          <EmptyState
            filter={statusFilter}
            onResetFilter={() => setStatusFilter("all")}
          />
        ) : (
          <Card>
            <CardContent className="p-0">
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="border-b border-gray-100 bg-gray-50/50">
                      <th className="px-6 py-3 text-left text-xs font-semibold tracking-wider text-gray-500 uppercase">
                        Organisation
                      </th>
                      <th className="px-6 py-3 text-left text-xs font-semibold tracking-wider text-gray-500 uppercase">
                        Inhaber
                      </th>
                      <th className="px-6 py-3 text-left text-xs font-semibold tracking-wider text-gray-500 uppercase">
                        Status
                      </th>
                      <th className="px-6 py-3 text-left text-xs font-semibold tracking-wider text-gray-500 uppercase">
                        Erstellt
                      </th>
                      <th className="px-6 py-3 text-right text-xs font-semibold tracking-wider text-gray-500 uppercase">
                        Aktionen
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {filteredOrganizations.map((org) => (
                      <OrganizationRow
                        key={org.id}
                        org={org}
                        actionLoading={actionLoading}
                        onApprove={handleApprove}
                        onReject={(id) => setShowRejectModal(id)}
                        onSuspend={handleSuspend}
                        onReactivate={handleReactivate}
                      />
                    ))}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        )}

        {/* Reject Modal */}
        <RejectModal
          isOpen={showRejectModal !== null}
          onClose={() => setShowRejectModal(null)}
          onConfirm={(reason) => {
            if (showRejectModal) {
              void handleReject(showRejectModal, reason);
            }
          }}
          isLoading={actionLoading === showRejectModal}
        />
      </div>
    </div>
  );
}

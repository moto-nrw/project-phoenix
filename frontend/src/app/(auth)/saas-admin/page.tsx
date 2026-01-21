"use client";

import { useCallback, useEffect, useState } from "react";
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

const STATUS_LABELS: Record<Organization["status"], string> = {
  pending: "Ausstehend",
  active: "Aktiv",
  rejected: "Abgelehnt",
  suspended: "Gesperrt",
};

const STATUS_COLORS: Record<Organization["status"], string> = {
  pending: "bg-yellow-100 text-yellow-800",
  active: "bg-green-100 text-green-800",
  rejected: "bg-red-100 text-red-800",
  suspended: "bg-gray-100 text-gray-800",
};

type StatusFilter = Organization["status"] | "all";

export default function SaasAdminPage() {
  const { data: session, isPending: isSessionLoading } = useSession();
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("pending");
  const [rejectReason, setRejectReason] = useState("");
  const [showRejectModal, setShowRejectModal] = useState<string | null>(null);

  const loadOrganizations = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const orgs = await fetchOrganizations(
        statusFilter === "all" ? undefined : statusFilter,
      );
      setOrganizations(orgs);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Fehler beim Laden");
    } finally {
      setLoading(false);
    }
  }, [statusFilter]);

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

  const handleReject = async (orgId: string) => {
    try {
      setActionLoading(orgId);
      await rejectOrganization(orgId, rejectReason || undefined);
      setShowRejectModal(null);
      setRejectReason("");
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
      <div className="flex min-h-screen items-center justify-center">
        <div className="rounded-lg bg-red-50 p-8 text-center">
          <h1 className="text-xl font-bold text-red-800">Nicht autorisiert</h1>
          <p className="mt-2 text-red-600">
            Du musst angemeldet sein, um diese Seite zu sehen.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto max-w-6xl px-4 py-8">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900">
          Organisation-Verwaltung
        </h1>
        <p className="mt-2 text-gray-600">
          Verwalte Organisation-Registrierungen und -Status
        </p>
      </div>

      {/* Status Filter */}
      <div className="mb-6 flex gap-2">
        {(
          [
            "pending",
            "active",
            "suspended",
            "rejected",
            "all",
          ] as StatusFilter[]
        ).map((status) => (
          <button
            key={status}
            onClick={() => setStatusFilter(status)}
            className={`rounded-lg px-4 py-2 text-sm font-medium transition-colors ${
              statusFilter === status
                ? "bg-blue-600 text-white"
                : "bg-gray-100 text-gray-700 hover:bg-gray-200"
            }`}
          >
            {status === "all" ? "Alle" : STATUS_LABELS[status]}
          </button>
        ))}
      </div>

      {/* Error Message */}
      {error && (
        <div className="mb-6 rounded-lg bg-red-50 p-4 text-red-700">
          {error}
          <button
            onClick={() => setError(null)}
            className="ml-4 text-sm underline"
          >
            Schließen
          </button>
        </div>
      )}

      {/* Organizations Table */}
      {loading ? (
        <Loading fullPage={false} />
      ) : organizations.length === 0 ? (
        <div className="rounded-lg bg-gray-50 p-8 text-center text-gray-600">
          Keine Organisationen gefunden.
        </div>
      ) : (
        <div className="overflow-hidden rounded-lg border border-gray-200 bg-white shadow">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium tracking-wider text-gray-500 uppercase">
                  Organisation
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium tracking-wider text-gray-500 uppercase">
                  Subdomain
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium tracking-wider text-gray-500 uppercase">
                  Inhaber
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium tracking-wider text-gray-500 uppercase">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium tracking-wider text-gray-500 uppercase">
                  Erstellt
                </th>
                <th className="px-6 py-3 text-right text-xs font-medium tracking-wider text-gray-500 uppercase">
                  Aktionen
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white">
              {organizations.map((org) => (
                <tr key={org.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="font-medium text-gray-900">{org.name}</div>
                    <div className="text-sm text-gray-500">{org.id}</div>
                  </td>
                  <td className="px-6 py-4 text-sm whitespace-nowrap text-gray-500">
                    {org.slug}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm text-gray-900">
                      {org.ownerName ?? "-"}
                    </div>
                    <div className="text-sm text-gray-500">
                      {org.ownerEmail ?? "-"}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`inline-flex rounded-full px-2 text-xs leading-5 font-semibold ${STATUS_COLORS[org.status]}`}
                    >
                      {STATUS_LABELS[org.status]}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm whitespace-nowrap text-gray-500">
                    {new Date(org.createdAt).toLocaleDateString("de-DE")}
                  </td>
                  <td className="px-6 py-4 text-right text-sm font-medium whitespace-nowrap">
                    <div className="flex justify-end gap-2">
                      {org.status === "pending" && (
                        <>
                          <button
                            onClick={() => handleApprove(org.id)}
                            disabled={actionLoading === org.id}
                            className="rounded bg-green-600 px-3 py-1 text-white hover:bg-green-700 disabled:opacity-50"
                          >
                            {actionLoading === org.id ? "..." : "Genehmigen"}
                          </button>
                          <button
                            onClick={() => setShowRejectModal(org.id)}
                            disabled={actionLoading === org.id}
                            className="rounded bg-red-600 px-3 py-1 text-white hover:bg-red-700 disabled:opacity-50"
                          >
                            Ablehnen
                          </button>
                        </>
                      )}
                      {org.status === "active" && (
                        <button
                          onClick={() => handleSuspend(org.id)}
                          disabled={actionLoading === org.id}
                          className="rounded bg-gray-600 px-3 py-1 text-white hover:bg-gray-700 disabled:opacity-50"
                        >
                          {actionLoading === org.id ? "..." : "Sperren"}
                        </button>
                      )}
                      {org.status === "suspended" && (
                        <button
                          onClick={() => handleReactivate(org.id)}
                          disabled={actionLoading === org.id}
                          className="rounded bg-blue-600 px-3 py-1 text-white hover:bg-blue-700 disabled:opacity-50"
                        >
                          {actionLoading === org.id ? "..." : "Reaktivieren"}
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Reject Modal */}
      {showRejectModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
            <h2 className="mb-4 text-lg font-bold text-gray-900">
              Organisation ablehnen
            </h2>
            <p className="mb-4 text-sm text-gray-600">
              Bitte gib einen Grund für die Ablehnung an (optional). Der Inhaber
              wird per E-Mail benachrichtigt.
            </p>
            <textarea
              value={rejectReason}
              onChange={(e) => setRejectReason(e.target.value)}
              placeholder="Grund für die Ablehnung..."
              className="mb-4 w-full rounded-lg border border-gray-300 p-3 text-sm focus:border-blue-500 focus:outline-none"
              rows={3}
            />
            <div className="flex justify-end gap-2">
              <button
                onClick={() => {
                  setShowRejectModal(null);
                  setRejectReason("");
                }}
                className="rounded-lg bg-gray-100 px-4 py-2 text-gray-700 hover:bg-gray-200"
              >
                Abbrechen
              </button>
              <button
                onClick={() => handleReject(showRejectModal)}
                disabled={actionLoading === showRejectModal}
                className="rounded-lg bg-red-600 px-4 py-2 text-white hover:bg-red-700 disabled:opacity-50"
              >
                {actionLoading === showRejectModal ? "..." : "Ablehnen"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

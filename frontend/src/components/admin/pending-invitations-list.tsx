"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { AlertTriangle, Mail } from "lucide-react";
import { useToast } from "~/contexts/ToastContext";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  listPendingInvitations,
  resendInvitation,
  revokeInvitation,
} from "~/lib/invitation-api";
import type { PendingInvitation } from "~/lib/invitation-helpers";
import type { ApiError } from "~/lib/auth-api";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import { isValidDateString, isDateExpired } from "~/lib/utils/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PendingInvitationsList" });

interface PendingInvitationsListProps {
  readonly refreshKey: number;
}

export function PendingInvitationsList({
  refreshKey,
}: PendingInvitationsListProps) {
  const [invitations, setInvitations] = useState<PendingInvitation[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<number | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<PendingInvitation | null>(
    null,
  );
  const { success: toastSuccess } = useToast();

  const loadInvitations = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await listPendingInvitations();
      setInvitations(data);
    } catch (err) {
      const apiError = err as ApiError | undefined;
      setError(
        apiError?.message ?? "Offene Einladungen konnten nicht geladen werden.",
      );
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    loadInvitations().catch((err) =>
      logger.error("failed to load invitations", {
        error: err instanceof Error ? err.message : String(err),
      }),
    );
  }, [loadInvitations, refreshKey]);

  const handleResend = async (id: number) => {
    setError(null);
    try {
      setActionLoading(id);
      await resendInvitation(id);
      toastSuccess("Einladung wurde erneut gesendet.");
      await loadInvitations();
    } catch (err) {
      const apiError = err as ApiError | undefined;
      setError(
        apiError?.message ??
          "Die Einladung konnte nicht erneut gesendet werden.",
      );
    } finally {
      setActionLoading(null);
    }
  };

  const handleRevoke = async () => {
    if (!revokeTarget) return;
    setError(null);
    try {
      setActionLoading(revokeTarget.id);
      await revokeInvitation(revokeTarget.id);
      toastSuccess("Einladung wurde widerrufen.");
      setRevokeTarget(null);
      await loadInvitations();
    } catch (err) {
      const apiError = err as ApiError | undefined;
      setError(
        apiError?.message ?? "Die Einladung konnte nicht widerrufen werden.",
      );
    } finally {
      setActionLoading(null);
    }
  };

  const sortedInvitations = useMemo(
    () =>
      [...invitations].sort(
        (a, b) =>
          new Date(a.expiresAt).getTime() - new Date(b.expiresAt).getTime(),
      ),
    [invitations],
  );

  if (isLoading) {
    return (
      <div className="flex items-center justify-center rounded-2xl border border-gray-200/50 bg-white/90 p-12 backdrop-blur-sm">
        <div className="flex items-center gap-3 text-sm text-gray-600">
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-gray-200 border-t-gray-900"></div>
          Wird geladen…
        </div>
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-gray-200/50 bg-white/90 p-3 shadow-sm backdrop-blur-sm md:p-4">
      <div className="mb-2 flex items-center gap-2">
        <div className="rounded-lg bg-gray-100 p-1.5">
          <Mail className="h-4 w-4 text-gray-500" aria-hidden="true" />
        </div>
        <h2 className="text-sm font-semibold text-gray-900">
          Offene Einladungen
        </h2>
        <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-500">
          {invitations.length}
        </span>
      </div>

      {error && (
        <div className="border-moto-red/20 bg-moto-red-soft mb-4 rounded-xl border p-3">
          <div className="flex items-start justify-between gap-2">
            <div className="flex items-start gap-2">
              <AlertTriangle
                className="text-moto-red mt-0.5 h-4 w-4 flex-shrink-0"
                aria-hidden="true"
              />
              <p className="text-moto-red-strong text-sm">{error}</p>
            </div>
            <button
              type="button"
              onClick={() => loadInvitations()}
              className="bg-moto-red/10 text-moto-red-strong hover:bg-moto-red/20 rounded-lg px-2 py-1 text-xs font-medium"
            >
              Erneut versuchen
            </button>
          </div>
        </div>
      )}

      {/* Success toasts handled globally; no inline feedback */}

      {sortedInvitations.length === 0 ? (
        <div className="mt-2 rounded-xl border border-dashed border-gray-200 bg-gray-50/50 px-4 py-6 text-center">
          <p className="text-xs text-gray-400">Keine offenen Einladungen</p>
        </div>
      ) : (
        <div className="mt-4 overflow-x-auto rounded-xl border border-gray-200">
          <table className="min-w-full divide-y divide-gray-200 text-sm">
            <thead className="bg-gray-50/50">
              <tr>
                <th
                  scope="col"
                  className="px-3 py-2 text-left text-xs font-semibold text-gray-600 md:px-4 md:py-3"
                >
                  E-Mail
                </th>
                <th
                  scope="col"
                  className="hidden px-3 py-2 text-left text-xs font-semibold text-gray-600 sm:table-cell md:px-4 md:py-3"
                >
                  Rolle
                </th>
                <th
                  scope="col"
                  className="hidden px-3 py-2 text-left text-xs font-semibold text-gray-600 md:px-4 md:py-3 lg:table-cell"
                >
                  Von
                </th>
                <th
                  scope="col"
                  className="hidden px-3 py-2 text-left text-xs font-semibold text-gray-600 md:table-cell md:px-4 md:py-3"
                >
                  Gültig bis
                </th>
                <th
                  scope="col"
                  className="px-3 py-2 text-right text-xs font-semibold text-gray-600 md:px-4 md:py-3"
                >
                  Aktionen
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white">
              {sortedInvitations.map((invitation) => {
                const isValidDate = isValidDateString(invitation.expiresAt);
                const isExpired = isDateExpired(invitation.expiresAt);
                const expiresDate = isValidDate
                  ? new Date(invitation.expiresAt)
                  : null;

                return (
                  <tr
                    key={invitation.id}
                    className="transition-colors hover:bg-gray-50/50"
                  >
                    <td className="max-w-0 truncate px-3 py-2 text-xs font-medium text-gray-900 md:px-4 md:py-3 md:text-sm">
                      {invitation.email}
                    </td>
                    <td className="hidden truncate px-3 py-2 text-xs text-gray-600 sm:table-cell md:px-4 md:py-3 md:text-sm">
                      {getRoleDisplayName(invitation.roleName)}
                    </td>
                    <td className="hidden truncate px-3 py-2 text-xs text-gray-500 md:px-4 md:py-3 lg:table-cell">
                      {invitation.creatorEmail ??
                        (invitation.firstName && invitation.lastName
                          ? `${invitation.firstName} ${invitation.lastName}`
                          : "System")}
                    </td>
                    <td className="hidden px-3 py-2 whitespace-nowrap md:table-cell md:px-4 md:py-3">
                      {isValidDate && expiresDate ? (
                        <span
                          className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap md:px-2.5 md:py-1 ${isExpired ? "bg-moto-red-soft text-moto-red-strong" : "bg-gray-100 text-gray-700"}`}
                        >
                          {expiresDate.toLocaleDateString("de-DE", {
                            day: "2-digit",
                            month: "2-digit",
                            year: "numeric",
                          })}{" "}
                          {expiresDate.toLocaleTimeString("de-DE", {
                            hour: "2-digit",
                            minute: "2-digit",
                          })}
                        </span>
                      ) : (
                        <span className="text-xs text-gray-400">Ungültig</span>
                      )}
                    </td>
                    <td className="px-3 py-2 text-right whitespace-nowrap md:px-4 md:py-3">
                      <div className="flex justify-end gap-1 md:gap-2">
                        <button
                          type="button"
                          onClick={() => handleResend(invitation.id)}
                          disabled={
                            isExpired || actionLoading === invitation.id
                          }
                          className="min-h-[32px] rounded-lg bg-gray-100 px-2 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-50 md:min-h-0 md:px-3 md:py-1.5"
                        >
                          {actionLoading === invitation.id ? "…" : "Erneut"}
                        </button>
                        <button
                          type="button"
                          onClick={() => setRevokeTarget(invitation)}
                          disabled={actionLoading === invitation.id}
                          className="bg-moto-red-soft text-moto-red-strong hover:bg-moto-red/20 min-h-[32px] rounded-lg px-2 py-1.5 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50 md:min-h-0 md:px-3 md:py-1.5"
                        >
                          Löschen
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmationModal
        isOpen={!!revokeTarget}
        onClose={() => setRevokeTarget(null)}
        onConfirm={handleRevoke}
        title="Einladung widerrufen?"
        confirmText="Widerrufen"
        cancelText="Abbrechen"
        confirmButtonClass="bg-moto-red hover:bg-moto-red-hover text-white"
      >
        <p className="text-sm text-gray-600">
          Möchtest du die Einladung für{" "}
          <span className="font-medium text-gray-900">
            {revokeTarget?.email}
          </span>{" "}
          wirklich widerrufen? Der Link kann danach nicht mehr verwendet werden.
        </p>
      </ConfirmationModal>
    </div>
  );
}

"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useToast } from "~/contexts/ToastContext";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  listPendingInvitations,
  resendInvitation,
  revokeInvitation,
} from "~/lib/invitation-api";
import type { PendingInvitation } from "~/lib/invitation-helpers";
import type { ApiError } from "~/lib/auth-api";
import { isValidDateString, isDateExpired } from "~/lib/utils/date-helpers";

interface PendingInvitationsListProps {
  refreshKey: number;
}

export function PendingInvitationsList({
  refreshKey,
}: PendingInvitationsListProps) {
  const [invitations, setInvitations] = useState<PendingInvitation[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Local inline success feedback removed in favor of global toasts
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const [feedback, setFeedback] = useState<string | null>(null);
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
      console.error("Failed to load invitations", err),
    );
  }, [loadInvitations, refreshKey]);

  const handleResend = async (id: number) => {
    setFeedback(null);
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
    setFeedback(null);
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
    <div className="rounded-2xl border border-gray-200/50 bg-white/90 p-4 shadow-sm backdrop-blur-sm md:p-6">
      <div className="mb-4 flex items-center gap-2 md:gap-3">
        <div className="rounded-xl bg-gray-100 p-2">
          <svg
            className="h-4 w-4 text-gray-600 md:h-5 md:w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"
            />
          </svg>
        </div>
        <div className="flex-1">
          <h2 className="text-base font-semibold text-gray-900 md:text-lg">
            Offene Einladungen
          </h2>
          <p className="text-xs text-gray-600 md:text-sm">
            {invitations.length} offen
          </p>
        </div>
      </div>

      {error && (
        <div className="mb-4 rounded-xl border border-red-200/50 bg-red-50/50 p-3">
          <div className="flex items-start justify-between gap-2">
            <div className="flex items-start gap-2">
              <svg
                className="mt-0.5 h-4 w-4 flex-shrink-0 text-red-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
              <p className="text-sm text-red-700">{error}</p>
            </div>
            <button
              type="button"
              onClick={() => loadInvitations()}
              className="rounded-lg bg-red-100 px-2 py-1 text-xs font-medium text-red-700 hover:bg-red-200"
            >
              Erneut versuchen
            </button>
          </div>
        </div>
      )}

      {/* Success toasts handled globally; no inline feedback */}

      {sortedInvitations.length === 0 ? (
        <div className="mt-4 rounded-xl border border-dashed border-gray-200 bg-gray-50/50 px-4 py-16 text-center md:px-8 md:py-32">
          <svg
            className="mx-auto mb-3 h-10 w-10 text-gray-400 md:h-12 md:w-12"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={1.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
            />
          </svg>
          <p className="text-xs text-gray-600 md:text-sm">
            Keine offenen Einladungen
          </p>
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
              {sortedInvitations.map((invitation, index) => {
                const isValidDate = isValidDateString(invitation.expiresAt);
                const isExpired = isDateExpired(invitation.expiresAt);
                const expiresDate = isValidDate
                  ? new Date(invitation.expiresAt)
                  : null;

                return (
                  <tr
                    key={`${invitation.id}-${invitation.email}-${index}`}
                    className="transition-colors hover:bg-gray-50/50"
                  >
                    <td className="max-w-0 truncate px-3 py-2 text-xs font-medium text-gray-900 md:px-4 md:py-3 md:text-sm">
                      {invitation.email}
                    </td>
                    <td className="hidden truncate px-3 py-2 text-xs text-gray-600 sm:table-cell md:px-4 md:py-3 md:text-sm">
                      {invitation.roleName}
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
                          className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap md:px-2.5 md:py-1 ${isExpired ? "bg-red-50 text-red-700" : "bg-gray-100 text-gray-700"}`}
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
                          className="min-h-[32px] rounded-lg bg-red-50 px-2 py-1.5 text-xs font-medium text-red-700 transition-colors hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-50 md:min-h-0 md:px-3 md:py-1.5"
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
        confirmButtonClass="bg-red-600 hover:bg-red-700"
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

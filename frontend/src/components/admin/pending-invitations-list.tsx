"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  listPendingInvitations,
  resendInvitation,
  revokeInvitation,
} from "~/lib/invitation-api";
import type { PendingInvitation } from "~/lib/invitation-helpers";
import type { ApiError } from "~/lib/auth-api";

interface PendingInvitationsListProps {
  refreshKey: number;
}

export function PendingInvitationsList({ refreshKey }: PendingInvitationsListProps) {
  const [invitations, setInvitations] = useState<PendingInvitation[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<number | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<PendingInvitation | null>(null);

  const loadInvitations = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await listPendingInvitations();
      setInvitations(data);
    } catch (err) {
      const apiError = err as ApiError | undefined;
      setError(apiError?.message ?? "Offene Einladungen konnten nicht geladen werden.");
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    loadInvitations().catch((err) => console.error("Failed to load invitations", err));
  }, [loadInvitations, refreshKey]);

  const handleResend = async (id: number) => {
    setFeedback(null);
    setError(null);
    try {
      setActionLoading(id);
      await resendInvitation(id);
      setFeedback("Einladung wurde erneut gesendet.");
      await loadInvitations();
    } catch (err) {
      const apiError = err as ApiError | undefined;
      setError(apiError?.message ?? "Die Einladung konnte nicht erneut gesendet werden.");
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
      setFeedback("Einladung wurde widerrufen.");
      setRevokeTarget(null);
      await loadInvitations();
    } catch (err) {
      const apiError = err as ApiError | undefined;
      setError(apiError?.message ?? "Die Einladung konnte nicht widerrufen werden.");
    } finally {
      setActionLoading(null);
    }
  };

  const sortedInvitations = useMemo(
    () => [...invitations].sort((a, b) => new Date(a.expiresAt).getTime() - new Date(b.expiresAt).getTime()),
    [invitations]
  );

  if (isLoading) {
    return (
      <div className="flex items-center justify-center rounded-2xl border border-gray-200/50 bg-white/90 backdrop-blur-sm p-12">
        <div className="flex items-center gap-3 text-sm text-gray-600">
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-gray-200 border-t-gray-900"></div>
          Wird geladen…
        </div>
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-gray-200/50 bg-white/90 backdrop-blur-sm p-6 shadow-sm">
      <div className="flex items-center gap-3 mb-4">
        <div className="rounded-xl bg-gray-100 p-2">
          <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
          </svg>
        </div>
        <div className="flex-1">
          <h2 className="text-lg font-semibold text-gray-900">Offene Einladungen</h2>
          <p className="text-sm text-gray-600">{invitations.length} offen</p>
        </div>
      </div>

      {error && (
        <div className="mb-4 rounded-xl border border-red-200/50 bg-red-50/50 p-3">
          <div className="flex items-start gap-2">
            <svg className="h-4 w-4 text-red-600 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <p className="text-sm text-red-700">{error}</p>
          </div>
        </div>
      )}

      {feedback && (
        <div className="mb-4 rounded-xl border border-green-200/50 bg-green-50/50 p-3">
          <div className="flex items-start gap-2">
            <svg className="h-4 w-4 text-green-600 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <p className="text-sm text-green-700">{feedback}</p>
          </div>
        </div>
      )}

      {sortedInvitations.length === 0 ? (
        <div className="mt-4 rounded-xl border border-dashed border-gray-200 bg-gray-50/50 p-8 text-center">
          <svg className="mx-auto h-12 w-12 text-gray-400 mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
          </svg>
          <p className="text-sm text-gray-600">Keine offenen Einladungen</p>
        </div>
      ) : (
        <div className="mt-4 overflow-hidden rounded-xl border border-gray-200">
          <table className="min-w-full divide-y divide-gray-200 text-sm">
            <thead className="bg-gray-50/50">
              <tr>
                <th scope="col" className="px-4 py-3 text-left text-xs font-semibold text-gray-600">E-Mail</th>
                <th scope="col" className="px-4 py-3 text-left text-xs font-semibold text-gray-600">Rolle</th>
                <th scope="col" className="px-4 py-3 text-left text-xs font-semibold text-gray-600">Von</th>
                <th scope="col" className="px-4 py-3 text-left text-xs font-semibold text-gray-600">Gültig bis</th>
                <th scope="col" className="px-4 py-3 text-right text-xs font-semibold text-gray-600">Aktionen</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white">
              {sortedInvitations.map((invitation) => {
                const expiresDate = new Date(invitation.expiresAt);
                const isExpired = expiresDate.getTime() < Date.now();
                return (
                  <tr key={invitation.id} className="hover:bg-gray-50/50 transition-colors">
                    <td className="px-4 py-3 font-medium text-gray-900">{invitation.email}</td>
                    <td className="px-4 py-3 text-gray-600">{invitation.roleName}</td>
                    <td className="px-4 py-3 text-gray-500 text-xs">{invitation.creatorEmail ?? `ID ${invitation.createdBy}`}</td>
                    <td className="px-4 py-3">
                      <span className={`inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium ${isExpired ? "bg-red-50 text-red-700" : "bg-gray-100 text-gray-700"}`}>
                        {expiresDate.toLocaleString("de-DE")}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex justify-end gap-2">
                        <button
                          type="button"
                          onClick={() => handleResend(invitation.id)}
                          disabled={isExpired || actionLoading === invitation.id}
                          className="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          {actionLoading === invitation.id ? "…" : "Erneut"}
                        </button>
                        <button
                          type="button"
                          onClick={() => setRevokeTarget(invitation)}
                          disabled={actionLoading === invitation.id}
                          className="rounded-lg bg-red-50 px-3 py-1.5 text-xs font-medium text-red-700 transition-colors hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-50"
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
          Möchtest du die Einladung für <span className="font-medium text-gray-900">{revokeTarget?.email}</span> wirklich widerrufen?
          Der Link kann danach nicht mehr verwendet werden.
        </p>
      </ConfirmationModal>
    </div>
  );
}

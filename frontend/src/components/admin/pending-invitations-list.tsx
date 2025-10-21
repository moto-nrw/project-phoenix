"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert } from "~/components/ui";
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
      <div className="flex items-center justify-center rounded-3xl border border-gray-100 bg-white/80 p-6 shadow-inner">
        <div className="flex items-center gap-3 text-sm text-gray-600">
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-gray-200 border-t-[#5080d8]"></div>
          Einladungen werden geladen …
        </div>
      </div>
    );
  }

  return (
    <div className="rounded-3xl border border-gray-100 bg-white/90 p-6 shadow-xl backdrop-blur-xl">
      <div className="mb-4 flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold text-gray-900">Offene Einladungen</h2>
          <p className="text-sm text-gray-600">Verwalte offene Einladungen, sende Erinnerungen und widerrufe nicht mehr benötigte Links.</p>
        </div>
        <span className="rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-600">
          {invitations.length} offen
        </span>
      </div>

      <div className="space-y-3">
        {error && <Alert type="error" message={error} />}
        {feedback && <Alert type="success" message={feedback} />}
      </div>

      {sortedInvitations.length === 0 ? (
        <div className="mt-6 rounded-2xl border border-dashed border-gray-200 bg-gray-50/60 p-8 text-center text-sm text-gray-600">
          Es sind aktuell keine offenen Einladungen vorhanden.
        </div>
      ) : (
        <div className="mt-4 overflow-hidden rounded-2xl border border-gray-100">
          <table className="min-w-full divide-y divide-gray-100 text-sm">
            <thead className="bg-gray-50">
              <tr>
                <th scope="col" className="px-4 py-3 text-left font-semibold text-gray-600">E-Mail</th>
                <th scope="col" className="px-4 py-3 text-left font-semibold text-gray-600">Rolle</th>
                <th scope="col" className="px-4 py-3 text-left font-semibold text-gray-600">Erstellt von</th>
                <th scope="col" className="px-4 py-3 text-left font-semibold text-gray-600">Gültig bis</th>
                <th scope="col" className="px-4 py-3 text-right font-semibold text-gray-600">Aktionen</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 bg-white">
              {sortedInvitations.map((invitation) => {
                const expiresDate = new Date(invitation.expiresAt);
                const isExpired = expiresDate.getTime() < Date.now();
                return (
                  <tr key={invitation.id} className="hover:bg-gray-50/80">
                    <td className="px-4 py-3 font-medium text-gray-900">{invitation.email}</td>
                    <td className="px-4 py-3 text-gray-600">{invitation.roleName}</td>
                    <td className="px-4 py-3 text-gray-500">{invitation.creatorEmail ?? `ID ${invitation.createdBy}`}</td>
                    <td className="px-4 py-3">
                      <span className={`inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold ${isExpired ? "bg-red-50 text-red-600" : "bg-emerald-50 text-emerald-600"}`}>
                        {expiresDate.toLocaleString("de-DE")}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex justify-end gap-2">
                        <button
                          type="button"
                          onClick={() => handleResend(invitation.id)}
                          disabled={isExpired || actionLoading === invitation.id}
                          className="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:border-gray-300 hover:bg-gray-100 disabled:cursor-not-allowed disabled:border-gray-100 disabled:text-gray-400"
                        >
                          {actionLoading === invitation.id ? "Sende …" : "Erneut senden"}
                        </button>
                        <button
                          type="button"
                          onClick={() => setRevokeTarget(invitation)}
                          disabled={actionLoading === invitation.id}
                          className="rounded-lg border border-red-200 px-3 py-1.5 text-xs font-medium text-red-600 transition-colors hover:border-red-300 hover:bg-red-50 disabled:cursor-not-allowed disabled:border-gray-100 disabled:text-gray-300"
                        >
                          Widerrufen
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

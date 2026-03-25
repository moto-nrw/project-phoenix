"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useToast } from "~/contexts/ToastContext";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  listInvitations,
  resendInvitation,
  revokeInvitation,
} from "~/lib/invitation-api";
import type {
  InvitationRecord,
  InvitationStatus,
} from "~/lib/invitation-helpers";
import type { ApiError } from "~/lib/auth-api";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "InvitationDashboard" });

interface PendingInvitationsListProps {
  readonly refreshKey: number;
}

type FilterKey = "all" | "open" | "expired" | "accepted" | "revoked" | "failed";

const FILTER_LABELS: Record<FilterKey, string> = {
  all: "Alle",
  open: "Offen",
  expired: "Abgelaufen",
  accepted: "Angenommen",
  revoked: "Widerrufen",
  failed: "Fehlgeschlagen",
};

const STATUS_LABELS: Record<InvitationStatus, string> = {
  pending: "Offen",
  failed: "Fehlgeschlagen",
  expired: "Abgelaufen",
  accepted: "Angenommen",
  revoked: "Widerrufen",
};

const STATUS_STYLES: Record<InvitationStatus, string> = {
  pending: "bg-blue-50 text-blue-700",
  failed: "bg-amber-50 text-amber-700",
  expired: "bg-red-50 text-red-700",
  accepted: "bg-green-50 text-green-700",
  revoked: "bg-gray-100 text-gray-700",
};

const dateFormatter = new Intl.DateTimeFormat("de-DE", {
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});

function formatDate(value?: string | null): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Ungültig";
  return dateFormatter.format(date);
}

function matchesFilter(
  invitation: InvitationRecord,
  filter: FilterKey,
): boolean {
  const status = invitation.status ?? "pending";
  switch (filter) {
    case "open":
      return status === "pending" || status === "failed";
    case "expired":
      return status === "expired";
    case "accepted":
      return status === "accepted";
    case "revoked":
      return status === "revoked";
    case "failed":
      return status === "failed";
    case "all":
    default:
      return true;
  }
}

function canActOn(invitation: InvitationRecord): boolean {
  const status = invitation.status ?? "pending";
  return status === "pending" || status === "failed" || status === "expired";
}

export function PendingInvitationsList({
  refreshKey,
}: PendingInvitationsListProps) {
  const [invitations, setInvitations] = useState<InvitationRecord[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<number | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<InvitationRecord | null>(
    null,
  );
  const [filter, setFilter] = useState<FilterKey>("all");
  const { success: toastSuccess } = useToast();

  const loadInvitations = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await listInvitations();
      setInvitations(data);
    } catch (err) {
      const apiError = err as ApiError | undefined;
      setError(
        apiError?.message ?? "Einladungen konnten nicht geladen werden.",
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

  const filteredInvitations = useMemo(
    () => invitations.filter((invitation) => matchesFilter(invitation, filter)),
    [filter, invitations],
  );

  const counts = useMemo(() => {
    const c = {
      all: 0,
      open: 0,
      expired: 0,
      accepted: 0,
      revoked: 0,
      failed: 0,
    };
    for (const inv of invitations) {
      c.all++;
      const s = inv.status ?? "pending";
      if (s === "pending") c.open++;
      else if (s === "failed") {
        c.open++;
        c.failed++;
      } else if (s === "expired") c.expired++;
      else if (s === "accepted") c.accepted++;
      else if (s === "revoked") c.revoked++;
    }
    return c;
  }, [invitations]);

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
      <div className="mb-4 flex items-center gap-3">
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
            Einladungsstatus
          </h2>
          <p className="text-xs text-gray-600 md:text-sm">
            {invitations.length} Einladungen in den letzten 30 Tagen
          </p>
        </div>
      </div>

      <div className="mb-4 flex flex-wrap gap-2">
        {(Object.keys(FILTER_LABELS) as FilterKey[]).map((key) => (
          <button
            key={key}
            type="button"
            onClick={() => setFilter(key)}
            className={`rounded-full px-3 py-1.5 text-xs font-medium transition-colors ${
              filter === key
                ? "bg-gray-900 text-white"
                : "bg-gray-100 text-gray-700 hover:bg-gray-200"
            }`}
          >
            {FILTER_LABELS[key]} ({counts[key]})
          </button>
        ))}
      </div>

      {error && (
        <div className="mb-4 rounded-xl border border-red-200/50 bg-red-50/50 p-3">
          <div className="flex items-start justify-between gap-2">
            <p className="text-sm text-red-700">{error}</p>
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

      {filteredInvitations.length === 0 ? (
        <div className="mt-4 rounded-xl border border-dashed border-gray-200 bg-gray-50/50 px-4 py-16 text-center md:px-8 md:py-32">
          <p className="text-xs text-gray-600 md:text-sm">
            Keine Einladungen in dieser Ansicht
          </p>
        </div>
      ) : (
        <div className="mt-4 overflow-x-auto rounded-xl border border-gray-200">
          <table className="min-w-full divide-y divide-gray-200 text-sm">
            <thead className="bg-gray-50/50">
              <tr>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-600 md:px-4 md:py-3">
                  E-Mail
                </th>
                <th className="hidden px-3 py-2 text-left text-xs font-semibold text-gray-600 sm:table-cell md:px-4 md:py-3">
                  Rolle
                </th>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-600 md:px-4 md:py-3">
                  Status
                </th>
                <th className="hidden px-3 py-2 text-left text-xs font-semibold text-gray-600 md:px-4 md:py-3 lg:table-cell">
                  Erstellt
                </th>
                <th className="hidden px-3 py-2 text-left text-xs font-semibold text-gray-600 md:px-4 md:py-3 lg:table-cell">
                  Ablauf
                </th>
                <th className="hidden px-3 py-2 text-left text-xs font-semibold text-gray-600 md:px-4 md:py-3 xl:table-cell">
                  Zuletzt gesendet
                </th>
                <th className="px-3 py-2 text-right text-xs font-semibold text-gray-600 md:px-4 md:py-3">
                  Aktionen
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white">
              {filteredInvitations.map((invitation) => (
                <tr
                  key={invitation.id}
                  className="transition-colors hover:bg-gray-50/50"
                >
                  <td className="max-w-0 truncate px-3 py-2 text-xs font-medium text-gray-900 md:px-4 md:py-3 md:text-sm">
                    <div>{invitation.email}</div>
                    <div className="mt-1 text-[11px] text-gray-500">
                      {invitation.creatorEmail ?? "System"}
                    </div>
                    {invitation.emailError && (
                      <div className="mt-1 text-[11px] text-red-600">
                        {invitation.emailError}
                      </div>
                    )}
                  </td>
                  <td className="hidden truncate px-3 py-2 text-xs text-gray-600 sm:table-cell md:px-4 md:py-3 md:text-sm">
                    {getRoleDisplayName(invitation.roleName)}
                  </td>
                  <td className="px-3 py-2 whitespace-nowrap md:px-4 md:py-3">
                    <div className="flex flex-col gap-1">
                      <span
                        className={`inline-flex w-fit items-center rounded-full px-2.5 py-1 text-xs font-medium ${STATUS_STYLES[invitation.status ?? "pending"]}`}
                      >
                        {STATUS_LABELS[invitation.status ?? "pending"]}
                      </span>
                      <span className="text-[11px] text-gray-500">
                        Versand: {invitation.deliveryStatus ?? "pending"}
                        {(invitation.emailRetryCount ?? 0) > 0
                          ? ` · Versuche ${invitation.emailRetryCount ?? 0}`
                          : ""}
                      </span>
                    </div>
                  </td>
                  <td className="hidden px-3 py-2 text-xs text-gray-600 md:px-4 md:py-3 lg:table-cell">
                    {formatDate(invitation.createdAt)}
                  </td>
                  <td className="hidden px-3 py-2 text-xs text-gray-600 md:px-4 md:py-3 lg:table-cell">
                    {formatDate(invitation.expiresAt)}
                  </td>
                  <td className="hidden px-3 py-2 text-xs text-gray-600 md:px-4 md:py-3 xl:table-cell">
                    {formatDate(invitation.emailSentAt)}
                  </td>
                  <td className="px-3 py-2 text-right whitespace-nowrap md:px-4 md:py-3">
                    <div className="flex justify-end gap-1 md:gap-2">
                      <button
                        type="button"
                        onClick={() => handleResend(invitation.id)}
                        disabled={
                          !canActOn(invitation) ||
                          actionLoading === invitation.id
                        }
                        className="min-h-[32px] rounded-lg bg-gray-100 px-2 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-50 md:px-3"
                      >
                        {actionLoading === invitation.id ? "…" : "Erneut"}
                      </button>
                      <button
                        type="button"
                        onClick={() => setRevokeTarget(invitation)}
                        disabled={
                          !canActOn(invitation) ||
                          actionLoading === invitation.id
                        }
                        className="min-h-[32px] rounded-lg bg-red-50 px-2 py-1.5 text-xs font-medium text-red-700 transition-colors hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-3"
                      >
                        Widerrufen
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
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

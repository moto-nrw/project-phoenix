"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Mail, Send, Trash2 } from "lucide-react";
import {
  useApiErrorDisplay,
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formErrorMessage } from "~/components/ui/form-error";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import {
  listPendingInvitations,
  resendInvitation,
  revokeInvitation,
} from "~/lib/invitation-api";
import type { PendingInvitation } from "~/lib/invitation-helpers";
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
  // Ladefehler stehen in der Karte, mit Wiederholen (#2517).
  const [loadFailed, setLoadFailed] = useState(false);
  const loadError = useApiLoadError();
  const showLoadError = loadError.show;
  const clearLoadError = loadError.clear;
  const latestLoadRef = useRef<() => void>(() => undefined);
  const [actionLoading, setActionLoading] = useState<number | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<PendingInvitation | null>(
    null,
  );
  const { success: toastSuccess } = useToast();
  // „Erneut senden“ ist eine Zeilenaktion ohne Formular: Fehler als Toast.
  const { show: showActionError } = useApiErrorDisplay();
  // Widerrufen läuft im Bestätigungsdialog; sein Fehler bleibt im Dialog.
  const revokeErrors = useApiFormError();
  const clearRevokeError = revokeErrors.clear;
  const latestRevokeRef = useRef<() => void>(() => undefined);

  // Never rejects: a failed load shows its error in the card.
  const loadInvitations = useCallback(async () => {
    try {
      setIsLoading(true);
      const data = await listPendingInvitations();
      setInvitations(data);
      setLoadFailed(false);
      clearLoadError();
    } catch (err) {
      logger.error("failed to load invitations", {
        error: err instanceof Error ? err.message : String(err),
      });
      setLoadFailed(true);
      void showLoadError(err, {
        object: "die Liste der offenen Einladungen",
        retry: () => latestLoadRef.current(),
      });
    } finally {
      setIsLoading(false);
    }
  }, [showLoadError, clearLoadError]);
  useLayoutEffect(() => {
    latestLoadRef.current = () => void loadInvitations();
  });

  useEffect(() => {
    void loadInvitations();
  }, [loadInvitations, refreshKey]);

  const handleResend = async (id: number) => {
    try {
      setActionLoading(id);
      await resendInvitation(id);
      toastSuccess("Die Einladung ist erneut gesendet.");
      await loadInvitations();
    } catch (err) {
      logger.error("invitation_resend_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showActionError(err, {
        object: "das erneute Senden der Einladung",
        retry: () => void handleResend(id),
      });
    } finally {
      setActionLoading(null);
    }
  };

  const handleRevoke = async () => {
    if (!revokeTarget) return;
    clearRevokeError();
    try {
      setActionLoading(revokeTarget.id);
      await revokeInvitation(revokeTarget.id);
      toastSuccess("Die Einladung ist widerrufen.");
      setRevokeTarget(null);
      await loadInvitations();
    } catch (err) {
      logger.error("invitation_revoke_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await revokeErrors.show(err, {
        object: "das Widerrufen der Einladung",
        retry: () => latestRevokeRef.current(),
      });
    } finally {
      setActionLoading(null);
    }
  };
  useLayoutEffect(() => {
    latestRevokeRef.current = () => void handleRevoke();
  });

  const closeRevoke = () => {
    clearRevokeError();
    setRevokeTarget(null);
  };

  const sortedInvitations = useMemo(
    () =>
      [...invitations].sort(
        (a, b) =>
          new Date(a.expiresAt).getTime() - new Date(b.expiresAt).getTime(),
      ),
    [invitations],
  );

  // Bis der Katalogtext des Ladefehlers da ist, bleibt der Ladezustand
  // stehen, nie „Keine offenen Einladungen“.
  const failedWithoutData = loadFailed && invitations.length === 0;
  if (
    isLoading ||
    (failedWithoutData && formErrorMessage(loadError.error) === null)
  ) {
    return (
      <div className="moto-content-surface flex items-center justify-center rounded-2xl border p-12 shadow-sm">
        <div className="flex items-center gap-3 text-sm text-gray-600">
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-gray-200 border-t-gray-900"></div>
          Wird geladen…
        </div>
      </div>
    );
  }

  return (
    <div className="moto-content-surface rounded-2xl border p-3 shadow-sm md:p-4">
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

      <LoadErrorAlert error={loadError.error} className="mb-4" />

      {failedWithoutData ? null : sortedInvitations.length === 0 ? (
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
                      {/* Zeilenaktionen nur im Kebab der Zeile (BAUARTEN-SPEC
                          Bauart 1 Regel 4). Löschen fragt weiter im
                          ConfirmationModal nach. */}
                      <div className="flex justify-end">
                        <OverflowMenu
                          ariaLabel={`Aktionen für ${invitation.email}`}
                          items={[
                            ...(isExpired
                              ? []
                              : [
                                  {
                                    label: "Erneut senden",
                                    icon: (
                                      <Send className="h-4 w-4" aria-hidden />
                                    ),
                                    disabled: actionLoading === invitation.id,
                                    onClick: () => handleResend(invitation.id),
                                  },
                                ]),
                            {
                              label: "Löschen",
                              icon: <Trash2 className="h-4 w-4" aria-hidden />,
                              destructive: true,
                              disabled: actionLoading === invitation.id,
                              onClick: () => setRevokeTarget(invitation),
                            },
                          ]}
                        />
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
        onClose={closeRevoke}
        onConfirm={handleRevoke}
        title="Einladung widerrufen?"
        confirmText="Widerrufen"
        cancelText="Abbrechen"
        confirmVariant="danger"
      >
        <p className="text-sm text-gray-600">
          Möchten Sie die Einladung für{" "}
          <span className="font-medium text-gray-900">
            {revokeTarget?.email}
          </span>{" "}
          wirklich widerrufen? Der Link kann danach nicht mehr verwendet werden.
        </p>
        <FormErrorAlert message={revokeErrors.error} className="mt-3" />
      </ConfirmationModal>
    </div>
  );
}

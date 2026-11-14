"use client";

import { EmptyState } from "~/components/ui/empty-state";
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
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ConfirmationModal } from "~/components/ui/modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import {
  listPendingInvitations,
  resendInvitation,
  revokeInvitation,
} from "~/lib/invitation-api";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  invitationMailNote,
  mailStatusCopy,
} from "~/lib/email-delivery-status";
import type { PendingInvitation } from "~/lib/invitation-helpers";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import { isValidDateString, isDateExpired } from "~/lib/utils/date-helpers";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import { Loading } from "~/components/ui/loading";
import { SectionCard } from "~/components/ui/section-card";

const logger = createLogger({ component: "PendingInvitationsList" });

interface PendingInvitationsListProps {
  readonly refreshKey: number;
  /** Nach dem Widerrufen: eine externe Person steht wieder auf „Kein
   * Konto“ (#4573), die Personalliste lädt ihren Stand neu. */
  readonly onRevoked?: () => void;
}

function staffMailNote(invitation: PendingInvitation): string | null {
  return invitationMailNote(
    invitation,
    "Stimmt die Adresse nicht, löschen Sie die Einladung und laden Sie neu ein.",
  );
}

export function PendingInvitationsList({
  refreshKey,
  onRevoked,
}: PendingInvitationsListProps) {
  const [invitations, setInvitations] = useState<PendingInvitation[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  // Ladefehler stehen in der Karte, mit Wiederholen (#2517).
  const [loadFailed, setLoadFailed] = useState(false);
  const loadError = useApiLoadError();
  const showLoadError = loadError.show;
  const clearLoadError = loadError.clear;
  const latestLoadRef = useRef<() => void>(() => undefined);
  const latestRequest = useRef(0);
  const [actionLoading, setActionLoading] = useState<ReadonlySet<number>>(
    new Set(),
  );
  const setInvitationLoading = (id: number, loading: boolean) => {
    setActionLoading((current) => {
      const next = new Set(current);
      if (loading) next.add(id);
      else next.delete(id);
      return next;
    });
  };
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

  // Rejects on failure: after a write the caller shows it as a reload of a
  // saved change (#6652).
  const fetchInvitations = useCallback(async () => {
    const request = ++latestRequest.current;
    try {
      const data = await listPendingInvitations();
      if (request !== latestRequest.current) return;
      setInvitations(data);
      setLoadFailed(false);
      clearLoadError();
    } catch (error) {
      if (request !== latestRequest.current) return;
      throw error;
    } finally {
      if (request === latestRequest.current) setIsLoading(false);
    }
  }, [clearLoadError]);
  // Never rejects: a failed load shows its error in the card.
  const loadInvitations = useCallback(async () => {
    const request = latestRequest.current + 1;
    try {
      setIsLoading(true);
      await fetchInvitations();
    } catch (err) {
      if (request !== latestRequest.current) return;
      logger.error("failed to load invitations", {
        error: err instanceof Error ? err.message : String(err),
      });
      setLoadFailed(true);
      void showLoadError(err, {
        object: "die Liste der offenen Einladungen",
        retry: () => latestLoadRef.current(),
      });
    }
  }, [showLoadError, fetchInvitations]);
  useLayoutEffect(() => {
    latestLoadRef.current = () => void loadInvitations();
  });

  useEffect(() => {
    void loadInvitations();
  }, [loadInvitations, refreshKey]);
  const afterWrite = useRefreshAfterWrite(
    "die Anzeige der offenen Einladungen",
    fetchInvitations,
  );

  const handleResend = async (id: number) => {
    if (actionLoading.has(id)) return;
    try {
      setInvitationLoading(id, true);
      await resendInvitation(id);
    } catch (err) {
      logger.error("invitation_resend_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showActionError(err, {
        object: "das erneute Senden der Einladung",
        retry: () => void handleResend(id),
      });
      setInvitationLoading(id, false);
      return;
    }
    toastSuccess("Die Einladung ist erneut gesendet.");
    await afterWrite.refresh();
    setInvitationLoading(id, false);
  };

  const handleRevoke = async () => {
    if (!revokeTarget) return;
    clearRevokeError();
    try {
      setInvitationLoading(revokeTarget.id, true);
      await revokeInvitation(revokeTarget.id);
    } catch (err) {
      logger.error("invitation_revoke_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await revokeErrors.show(err, {
        object: "das Widerrufen der Einladung",
        retry: () => latestRevokeRef.current(),
      });
      setInvitationLoading(revokeTarget.id, false);
      return;
    }
    setInvitationLoading(revokeTarget.id, false);
    toastSuccess("Die Einladung ist widerrufen.");
    setRevokeTarget(null);
    onRevoked?.();
    await afterWrite.refresh();
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

  const columns: DataTableColumn<PendingInvitation>[] = [
    {
      key: "email",
      header: "E-Mail",
      className: "w-1/3 max-w-0 font-medium max-sm:w-full",
      render: (invitation) => {
        const mailCopy = mailStatusCopy(
          invitation.mailOutcome,
          invitation.addressMark,
          false,
          invitation.providerBlock,
        );
        return (
          <>
            <span className="block truncate">{invitation.email}</span>
            {mailCopy && (
              <span className="mt-1 block">
                <StatusBadge
                  label={mailCopy.label}
                  tone={mailCopy.tone}
                  compact
                />
              </span>
            )}
          </>
        );
      },
    },
    {
      key: "role",
      header: "Rolle",
      headerClassName: "hidden sm:table-cell",
      className: "hidden truncate sm:table-cell",
      render: (invitation) => (
        <span className="text-gray-600">
          {getRoleDisplayName(invitation.roleName)}
        </span>
      ),
    },
    {
      key: "creator",
      header: "Von",
      headerClassName: "hidden lg:table-cell",
      className: "hidden truncate lg:table-cell",
      render: (invitation) => (
        <span className="text-xs text-gray-500">
          {invitation.creatorEmail ??
            (invitation.firstName && invitation.lastName
              ? `${invitation.firstName} ${invitation.lastName}`
              : "System")}
        </span>
      ),
    },
    {
      key: "expires",
      header: "Gültig bis",
      headerClassName: "hidden md:table-cell",
      className: "hidden whitespace-nowrap md:table-cell",
      render: (invitation) => {
        if (!isValidDateString(invitation.expiresAt)) {
          return <span className="text-xs text-gray-400">Ungültig</span>;
        }
        const expiresDate = new Date(invitation.expiresAt);
        return (
          <StatusBadge
            label={`${expiresDate.toLocaleDateString("de-DE", {
              day: "2-digit",
              month: "2-digit",
              year: "numeric",
            })} ${expiresDate.toLocaleTimeString("de-DE", {
              hour: "2-digit",
              minute: "2-digit",
            })}`}
            tone={isDateExpired(invitation.expiresAt) ? "red" : "gray"}
          />
        );
      },
    },
    {
      key: "actions",
      header: "Aktionen",
      align: "right",
      className: "whitespace-nowrap",
      // Zeilenaktionen nur im Kebab der Zeile (BAUARTEN-SPEC Bauart 1 Regel 4).
      // Löschen fragt weiter im ConfirmationModal nach.
      render: (invitation) => (
        <div className="flex justify-end">
          <OverflowMenu
            ariaLabel={`Aktionen für ${invitation.email}`}
            items={[
              ...(isDateExpired(invitation.expiresAt) ||
              invitation.mailOutcome === "delayed" ||
              invitation.providerBlock === "active" ||
              invitation.providerBlock === "unreadable"
                ? []
                : [
                    {
                      label: "Erneut senden",
                      description:
                        "Schickt sofort eine neue E-Mail, der alte Link gilt nicht mehr.",
                      icon: <Send className="h-4 w-4" aria-hidden />,
                      disabled: actionLoading.has(invitation.id),
                      onClick: () => handleResend(invitation.id),
                    },
                  ]),
              {
                label: "Löschen",
                description:
                  "Macht den Einladungslink ungültig. moto fragt vorher nach.",
                icon: <Trash2 className="h-4 w-4" aria-hidden />,
                destructive: true,
                disabled: actionLoading.has(invitation.id),
                onClick: () => setRevokeTarget(invitation),
              },
            ]}
          />
        </div>
      ),
    },
  ];

  // Bis der Katalogtext des Ladefehlers da ist, bleibt der Ladezustand
  // stehen, nie „Keine offenen Einladungen“.
  const failedWithoutData = loadFailed && invitations.length === 0;
  if (
    isLoading ||
    (failedWithoutData && formErrorMessage(loadError.error) === null)
  ) {
    return (
      <SectionCard>
        <Loading message="Wird geladen…" />
      </SectionCard>
    );
  }

  return (
    <SectionCard
      title="Offene Einladungen"
      icon={Mail}
      titleBadge={
        <StatusBadge label={String(invitations.length)} tone="gray" compact />
      }
    >
      <LoadErrorAlert error={loadError.error} className="mb-4" />
      <LoadErrorAlert error={afterWrite.error} className="mb-4" />

      {failedWithoutData ? null : sortedInvitations.length === 0 ? (
        <EmptyState
          variant="dashed"
          title="Keine offenen Einladungen"
          className="mt-2"
        />
      ) : (
        <div className="mt-4">
          <DataTable
            columns={columns}
            rows={sortedInvitations}
            getRowKey={(invitation) => invitation.id}
            renderSubRows={(invitation) => {
              const mailNote = staffMailNote(invitation);
              return mailNote
                ? [{ key: `${invitation.id}-mail-note`, content: mailNote }]
                : [];
            }}
          />
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
    </SectionCard>
  );
}

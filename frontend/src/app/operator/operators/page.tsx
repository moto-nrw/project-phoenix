"use client";

import { operatorInvitationKeys } from "~/lib/swr/portal-keys";

import { useState, useCallback, useRef } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import { useSWRAuth } from "~/lib/swr";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import {
  createOperatorInvitation,
  listOperatorInvitations,
  resendOperatorInvitation,
  revokeOperatorInvitation,
  deactivateOperator,
  reactivateOperator,
  type OperatorInvitationsData,
} from "~/lib/operator/operator-invitation-api";
import type {
  PendingOperatorInvitation,
  OperatorInfo,
} from "~/lib/operator/operator-invitation-helpers";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { createLogger } from "~/lib/logger";
import { TenantPage } from "~/components/ui/tenant-page";
import { operatorsConcept } from "~/lib/concepts/operators";
import { SectionCard } from "~/components/ui/section-card";
import { DataTableStatusBadge } from "~/components/ui/data-table";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";

const logger = createLogger({ component: "OperatorOperatorsPage" });

export default function OperatorOperatorsPage() {
  useSetBreadcrumb({ pageTitle: "Operatoren" });

  const {
    data,
    error: fetchError,
    isLoading,
    mutate,
  } = useSWRAuth<OperatorInvitationsData>(
    operatorInvitationKeys.list(),
    () => listOperatorInvitations(),
    { keepPreviousData: false, revalidateOnFocus: false },
  );
  const loadError = useSwrLoadError(
    fetchError,
    "die Liste der Operatoren",
    () => void mutate(),
  );
  const [inviteOpen, setInviteOpen] = useState(false);
  const [revokeTarget, setRevokeTarget] =
    useState<PendingOperatorInvitation | null>(null);
  const [revokeLoading, setRevokeLoading] = useState(false);
  const {
    error: revokeError,
    show: showRevokeError,
    clear: clearRevokeError,
  } = useApiFormError();
  const [deactivateTarget, setDeactivateTarget] = useState<OperatorInfo | null>(
    null,
  );
  const [deactivateLoading, setDeactivateLoading] = useState(false);
  const {
    error: deactivateError,
    show: showDeactivateError,
    clear: clearDeactivateError,
  } = useApiFormError();
  const { success: toastSuccess } = useToast();

  const openRevoke = useCallback(
    (invitation: PendingOperatorInvitation) => {
      clearRevokeError();
      setRevokeTarget(invitation);
    },
    [clearRevokeError],
  );

  const handleRevoke = useCallback(async () => {
    if (!revokeTarget) return;
    clearRevokeError();
    setRevokeLoading(true);
    try {
      await revokeOperatorInvitation(revokeTarget.id);
      toastSuccess("Einladung wurde widerrufen.");
      setRevokeTarget(null);
      void mutate();
    } catch (err) {
      logger.error("revoke_invitation_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showRevokeError(err, { object: "das Widerrufen der Einladung" });
    } finally {
      setRevokeLoading(false);
    }
  }, [clearRevokeError, mutate, revokeTarget, showRevokeError, toastSuccess]);

  const openDeactivate = useCallback(
    (operator: OperatorInfo) => {
      clearDeactivateError();
      setDeactivateTarget(operator);
    },
    [clearDeactivateError],
  );

  const handleDeactivate = useCallback(async () => {
    if (!deactivateTarget) return;
    clearDeactivateError();
    setDeactivateLoading(true);
    try {
      await deactivateOperator(deactivateTarget.id);
      toastSuccess(`${deactivateTarget.displayName} wurde deaktiviert.`);
      setDeactivateTarget(null);
      void mutate();
    } catch (err) {
      logger.error("deactivate_operator_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showDeactivateError(err, {
        object: "das Deaktivieren des Kontos",
      });
    } finally {
      setDeactivateLoading(false);
    }
  }, [
    clearDeactivateError,
    deactivateTarget,
    mutate,
    showDeactivateError,
    toastSuccess,
  ]);

  return (
    <TenantPage
      title="Operatoren"
      info={<p>Neue Operatoren einladen und bestehende anzeigen.</p>}
      actions={
        <Button
          type="button"
          variant="primary"
          size="md"
          onClick={() => setInviteOpen(true)}
        >
          Neuen Operator einladen
        </Button>
      }
      loading={isLoading}
      loadingLabel="Operatoren werden geladen"
      overlays={
        <>
          <InviteOperatorModal
            isOpen={inviteOpen}
            onClose={() => setInviteOpen(false)}
            onCreated={() => void mutate()}
          />

          <ConfirmationModal
            isOpen={revokeTarget !== null}
            onClose={() => setRevokeTarget(null)}
            onConfirm={() => void handleRevoke()}
            title="Einladung widerrufen"
            confirmText="Widerrufen"
            confirmVariant="danger"
            isConfirmLoading={revokeLoading}
            isConfirmDisabled={revokeLoading}
          >
            <p>
              Möchtest du die Einladung an{" "}
              <strong>{revokeTarget?.email ?? ""}</strong> wirklich widerrufen?
            </p>
            <FormErrorAlert message={revokeError} className="mt-3" />
          </ConfirmationModal>

          <ConfirmationModal
            isOpen={deactivateTarget !== null}
            onClose={() => setDeactivateTarget(null)}
            onConfirm={() => void handleDeactivate()}
            title="Konto deaktivieren"
            confirmText="Deaktivieren"
            confirmVariant="danger"
            isConfirmLoading={deactivateLoading}
            isConfirmDisabled={deactivateLoading}
          >
            <p>
              Möchtest du das Konto von{" "}
              <strong>{deactivateTarget?.email ?? ""}</strong> deaktivieren?
            </p>
            <p className="mt-2">
              Die Person kann sich danach nicht mehr anmelden. Ohne erneute
              Aktivierung löscht moto das Konto nach Ende des nächsten Jahres.
            </p>
            <FormErrorAlert message={deactivateError} className="mt-3" />
          </ConfirmationModal>
        </>
      }
    >
      <LoadErrorAlert error={loadError} />

      {data && (
        <>
          {data.invitations.length > 0 && (
            <PendingInvitationsList
              invitations={data.invitations}
              onMutate={() => void mutate()}
              onRevoke={openRevoke}
            />
          )}
          <OperatorsList
            operators={data.operators}
            onMutate={() => void mutate()}
            onDeactivate={openDeactivate}
          />
        </>
      )}
    </TenantPage>
  );
}

// --- Invite Form ---

function InviteOperatorModal({
  isOpen,
  onClose,
  onCreated,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreated: () => void;
}) {
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showFormError, clear: clearFormError } = formErrors;
  const { success: toastSuccess } = useToast();

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!email.trim() || isSubmitting) return;

      setIsSubmitting(true);
      clearFormError();

      try {
        await createOperatorInvitation({
          email: email.trim(),
          displayName: displayName.trim() || undefined,
        });
        toastSuccess("Einladung wurde gesendet.");
        setEmail("");
        setDisplayName("");
        onCreated();
        onClose();
      } catch (err) {
        logger.error("create_invitation_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void showFormError(err, { object: "die Einladung" });
      } finally {
        setIsSubmitting(false);
      }
    },
    [
      email,
      displayName,
      isSubmitting,
      onCreated,
      onClose,
      toastSuccess,
      showFormError,
      clearFormError,
    ],
  );

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Neuen Operator einladen">
      <form
        ref={formRef}
        onSubmit={(e) => void handleSubmit(e)}
        className="space-y-4"
      >
        <FormErrorAlert message={formErrors.error} />
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            id="invite-email"
            name="email"
            type="email"
            label="E-Mail-Adresse"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="operator@example.com"
            error={formErrors.fieldError("email")}
            controlSize="compact"
          />
          <Input
            id="invite-display-name"
            name="display_name"
            type="text"
            label="Anzeigename (optional)"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="Max Mustermann"
            controlSize="compact"
          />
        </div>

        <Button
          type="submit"
          variant="primary"
          size="md"
          disabled={isSubmitting || !email.trim()}
        >
          {isSubmitting ? "Wird gesendet..." : "Einladung senden"}
        </Button>
      </form>
    </Modal>
  );
}

// --- Pending Invitations ---

function PendingInvitationsList({
  invitations,
  onMutate,
  onRevoke,
}: {
  readonly invitations: PendingOperatorInvitation[];
  readonly onMutate: () => void;
  readonly onRevoke: (invitation: PendingOperatorInvitation) => void;
}) {
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const { success: toastSuccess } = useToast();
  const { show: showResendError } = useApiErrorDisplay();

  const handleResend = useCallback(
    async (id: string) => {
      setActionLoading(id);
      try {
        await resendOperatorInvitation(id);
        toastSuccess("Einladung wurde erneut gesendet.");
        onMutate();
      } catch (err) {
        logger.error("resend_invitation_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void showResendError(err, {
          object: "das erneute Senden der Einladung",
          retry: () => void handleResend(id),
        });
      } finally {
        setActionLoading(null);
      }
    },
    [onMutate, toastSuccess, showResendError],
  );

  return (
    <SectionCard title="Offene Einladungen" concept={operatorsConcept}>
      <div className="space-y-3">
        {invitations.map((inv) => (
          <div
            key={inv.id}
            className="flex items-center justify-between rounded-lg border border-gray-100 bg-gray-50 px-4 py-3"
          >
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-gray-900">
                {inv.email}
              </p>
              <p className="text-xs text-gray-500">
                Eingeladen von {inv.creatorName ?? "Unbekannt"}
                {" — "}
                Läuft ab am{" "}
                {new Date(inv.expiresAt).toLocaleDateString("de-DE", {
                  timeZone: "Europe/Berlin",
                  day: "2-digit",
                  month: "2-digit",
                  year: "numeric",
                  hour: "2-digit",
                  minute: "2-digit",
                })}
              </p>
            </div>
            <div className="ml-4 flex gap-2">
              <Button
                type="button"
                variant="surface"
                size="compact"
                disabled={actionLoading === inv.id}
                onClick={() => void handleResend(inv.id)}
              >
                Erneut senden
              </Button>
              <Button
                type="button"
                variant="outline_danger"
                size="compact"
                disabled={actionLoading === inv.id}
                onClick={() => onRevoke(inv)}
              >
                Widerrufen
              </Button>
            </div>
          </div>
        ))}
      </div>
    </SectionCard>
  );
}

// --- Existing Operators List ---

const berlinDate: Intl.DateTimeFormatOptions = {
  timeZone: "Europe/Berlin",
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
};

function OperatorsList({
  operators,
  onMutate,
  onDeactivate,
}: {
  readonly operators: OperatorInfo[];
  readonly onMutate: () => void;
  readonly onDeactivate: (operator: OperatorInfo) => void;
}) {
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const { success: toastSuccess } = useToast();
  const { show: showReactivateError } = useApiErrorDisplay();

  const handleReactivate = useCallback(
    async (operator: OperatorInfo) => {
      setActionLoading(operator.id);
      try {
        await reactivateOperator(operator.id);
        toastSuccess(`${operator.displayName} kann sich wieder anmelden.`);
        onMutate();
      } catch (err) {
        logger.error("reactivate_operator_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void showReactivateError(err, {
          object: "das Aktivieren des Kontos",
          retry: () => void handleReactivate(operator),
        });
      } finally {
        setActionLoading(null);
      }
    },
    [onMutate, toastSuccess, showReactivateError],
  );

  return (
    <SectionCard title="Bestehende Operatoren" concept={operatorsConcept}>
      {operators.length === 0 ? (
        <p className="text-sm text-gray-500">Keine Operatoren vorhanden.</p>
      ) : (
        <div className="space-y-3">
          {operators.map((op) => (
            <div
              key={op.id}
              className="flex flex-col gap-2 rounded-lg border border-gray-100 bg-gray-50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
            >
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-gray-900">
                  {op.displayName}
                </p>
                <p className="truncate text-xs text-gray-500">{op.email}</p>
                {op.deactivatedAt && (
                  <p className="text-xs text-gray-500">
                    Deaktiviert am{" "}
                    {new Date(op.deactivatedAt).toLocaleDateString(
                      "de-DE",
                      berlinDate,
                    )}
                  </p>
                )}
                {op.deletionDueAt && (
                  <p className="text-xs text-gray-500">
                    Wird ab{" "}
                    {new Date(op.deletionDueAt).toLocaleDateString(
                      "de-DE",
                      berlinDate,
                    )}{" "}
                    gelöscht
                  </p>
                )}
              </div>
              <div className="flex items-center justify-between gap-2 sm:ml-4 sm:block sm:text-right">
                {op.lastLogin ? (
                  <p className="text-xs text-gray-400">
                    Letzter Login:{" "}
                    {new Date(op.lastLogin).toLocaleDateString(
                      "de-DE",
                      berlinDate,
                    )}
                  </p>
                ) : (
                  <p className="text-xs text-gray-400">Noch nicht angemeldet</p>
                )}
                <div className="flex items-center justify-end gap-2 sm:mt-1">
                  <DataTableStatusBadge active={op.active} />
                  {op.isSelf ? (
                    // Keeps the badge in line with the rows that carry a menu.
                    <span aria-hidden="true" className="size-6" />
                  ) : (
                    <OverflowMenu
                      ariaLabel={`Aktionen für ${op.displayName}`}
                      triggerSize="sm"
                      items={[
                        op.active
                          ? {
                              label: "Deaktivieren",
                              destructive: true,
                              disabled: actionLoading === op.id,
                              onClick: () => onDeactivate(op),
                            }
                          : {
                              label: "Wieder aktivieren",
                              disabled: actionLoading === op.id,
                              onClick: () => void handleReactivate(op),
                            },
                      ]}
                    />
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </SectionCard>
  );
}

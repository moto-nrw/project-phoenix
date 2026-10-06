"use client";

import { useState, useCallback, useRef } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
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
import { ConfirmationModal } from "~/components/ui/modal";
import {
  createOperatorInvitation,
  listOperatorInvitations,
  resendOperatorInvitation,
  revokeOperatorInvitation,
  type OperatorInvitationsData,
} from "~/lib/operator/operator-invitation-api";
import type {
  PendingOperatorInvitation,
  OperatorInfo,
} from "~/lib/operator/operator-invitation-helpers";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { createLogger } from "~/lib/logger";
import {
  ConceptPageHeader,
  SectionHeader,
} from "~/components/ui/concept-section-header";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { DataTableStatusBadge } from "~/components/ui/data-table";
import { SkeletonRegion, ListSkeleton } from "~/components/ui/page-skeletons";

const logger = createLogger({ component: "OperatorOperatorsPage" });

export default function OperatorOperatorsPage() {
  useSetBreadcrumb({ pageTitle: "Operatoren" });

  const {
    data,
    error: fetchError,
    isLoading,
    mutate,
  } = useSWR<OperatorInvitationsData>(
    "operator-invitations",
    () => listOperatorInvitations(),
    { revalidateOnFocus: false },
  );
  const loadError = useSwrLoadError(
    fetchError,
    "die Liste der Operatoren",
    () => void mutate(),
  );

  return (
    <div className="space-y-8 p-6">
      <ConceptPageHeader
        title="Operatoren"
        concept="operators"
        subtitle="Neue Operatoren einladen und bestehende anzeigen."
      />

      <InviteForm onCreated={() => void mutate()} />

      <LoadErrorAlert error={loadError} />

      {isLoading && (
        <SkeletonRegion label="Operatoren werden geladen">
          <ListSkeleton rows={4} avatar={false} />
        </SkeletonRegion>
      )}

      {data && (
        <>
          {data.invitations.length > 0 && (
            <PendingInvitationsList
              invitations={data.invitations}
              onMutate={() => void mutate()}
            />
          )}
          <OperatorsList operators={data.operators} />
        </>
      )}
    </div>
  );
}

// --- Invite Form ---

function InviteForm({ onCreated }: { readonly onCreated: () => void }) {
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
      toastSuccess,
      showFormError,
      clearFormError,
    ],
  );

  return (
    <div className="moto-content-surface rounded-xl border p-6 shadow-sm">
      <SectionHeader
        title="Neuen Operator einladen"
        icon={<MotoConceptIcon concept="operators" size={22} />}
        className="mb-4"
      />
      <form
        ref={formRef}
        onSubmit={(e) => void handleSubmit(e)}
        className="space-y-4"
      >
        <FormErrorAlert message={formErrors.error} />
        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <label
              htmlFor="invite-email"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              E-Mail-Adresse *
            </label>
            <input
              id="invite-email"
              name="email"
              type="email"
              aria-invalid={formErrors.fieldError("email") ? true : undefined}
              aria-describedby={
                formErrors.fieldError("email")
                  ? "invite-email-error"
                  : undefined
              }
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="operator@example.com"
              className="focus:ring-moto-purple w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
            />
            {formErrors.fieldError("email") ? (
              <p id="invite-email-error" className="text-moto-red mt-1 text-xs">
                {formErrors.fieldError("email")}
              </p>
            ) : null}
          </div>
          <div>
            <label
              htmlFor="invite-display-name"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Anzeigename (optional)
            </label>
            <input
              id="invite-display-name"
              name="display_name"
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="Max Mustermann"
              className="focus:ring-moto-purple w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
            />
          </div>
        </div>

        <button
          type="submit"
          disabled={isSubmitting || !email.trim()}
          className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {isSubmitting ? "Wird gesendet..." : "Einladung senden"}
        </button>
      </form>
    </div>
  );
}

// --- Pending Invitations ---

function PendingInvitationsList({
  invitations,
  onMutate,
}: {
  readonly invitations: PendingOperatorInvitation[];
  readonly onMutate: () => void;
}) {
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [revokeTarget, setRevokeTarget] =
    useState<PendingOperatorInvitation | null>(null);
  const { success: toastSuccess } = useToast();
  const { show: showResendError } = useApiErrorDisplay();
  const revokeErrors = useApiFormError();
  const { show: showRevokeError, clear: clearRevokeError } = revokeErrors;

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
    setActionLoading(revokeTarget.id);
    try {
      await revokeOperatorInvitation(revokeTarget.id);
      toastSuccess("Einladung wurde widerrufen.");
      setRevokeTarget(null);
      onMutate();
    } catch (err) {
      logger.error("revoke_invitation_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showRevokeError(err, { object: "das Widerrufen der Einladung" });
    } finally {
      setActionLoading(null);
    }
  }, [revokeTarget, onMutate, toastSuccess, showRevokeError, clearRevokeError]);

  return (
    <div className="moto-content-surface rounded-xl border p-6 shadow-sm">
      <SectionHeader
        title="Offene Einladungen"
        icon={<MotoConceptIcon concept="operators" size={22} />}
        className="mb-4"
      />

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
              <button
                type="button"
                disabled={actionLoading === inv.id}
                onClick={() => void handleResend(inv.id)}
                className="bg-moto-purple/15 text-moto-purple-strong hover:bg-moto-purple/25 rounded-full px-2 py-0.5 text-xs font-medium transition-colors disabled:opacity-50"
              >
                Erneut senden
              </button>
              <button
                type="button"
                disabled={actionLoading === inv.id}
                onClick={() => openRevoke(inv)}
                className="bg-moto-red/15 text-moto-red-strong hover:bg-moto-red/25 rounded-full px-2 py-0.5 text-xs font-medium transition-colors disabled:opacity-50"
              >
                Widerrufen
              </button>
            </div>
          </div>
        ))}
      </div>

      <ConfirmationModal
        isOpen={revokeTarget !== null}
        onClose={() => setRevokeTarget(null)}
        onConfirm={() => void handleRevoke()}
        title="Einladung widerrufen"
        confirmText="Widerrufen"
        confirmVariant="danger"
      >
        <p>
          Möchtest du die Einladung an{" "}
          <strong>{revokeTarget?.email ?? ""}</strong> wirklich widerrufen?
        </p>
        <FormErrorAlert message={revokeErrors.error} className="mt-3" />
      </ConfirmationModal>
    </div>
  );
}

// --- Existing Operators List ---

function OperatorsList({ operators }: { readonly operators: OperatorInfo[] }) {
  return (
    <div className="moto-content-surface rounded-xl border p-6 shadow-sm">
      <SectionHeader
        title="Bestehende Operatoren"
        icon={<MotoConceptIcon concept="operators" size={22} />}
        className="mb-4"
      />

      {operators.length === 0 ? (
        <p className="text-sm text-gray-500">Keine Operatoren vorhanden.</p>
      ) : (
        <div className="space-y-3">
          {operators.map((op) => (
            <div
              key={op.id}
              className="flex items-center justify-between rounded-lg border border-gray-100 bg-gray-50 px-4 py-3"
            >
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-gray-900">
                  {op.displayName}
                </p>
                <p className="truncate text-xs text-gray-500">{op.email}</p>
              </div>
              <div className="ml-4 text-right">
                {op.lastLogin ? (
                  <p className="text-xs text-gray-400">
                    Letzter Login:{" "}
                    {new Date(op.lastLogin).toLocaleDateString("de-DE", {
                      timeZone: "Europe/Berlin",
                      day: "2-digit",
                      month: "2-digit",
                      year: "numeric",
                    })}
                  </p>
                ) : (
                  <p className="text-xs text-gray-400">Noch nicht angemeldet</p>
                )}
                <div className="mt-1 flex justify-end">
                  <DataTableStatusBadge active={op.active} />
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

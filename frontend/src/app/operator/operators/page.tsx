"use client";

import { useState, useCallback } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import { useToast } from "~/contexts/ToastContext";
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
import { isOperatorApiError } from "~/lib/operator/api-helpers";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { createLogger } from "~/lib/logger";

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

  return (
    <div className="space-y-8 p-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Operatoren</h1>
        <p className="mt-1 text-sm text-gray-500">
          Neue Operatoren einladen und bestehende anzeigen.
        </p>
      </div>

      <InviteForm onCreated={() => void mutate()} />

      {fetchError && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">
          {isOperatorApiError(fetchError)
            ? fetchError.message
            : "Daten konnten nicht geladen werden."}
        </div>
      )}

      {isLoading && (
        <div className="flex items-center justify-center py-12">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-gray-200 border-t-gray-900" />
        </div>
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
  const [error, setError] = useState<string | null>(null);
  const { success: toastSuccess } = useToast();

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!email.trim() || isSubmitting) return;

      setIsSubmitting(true);
      setError(null);

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
        const message = isOperatorApiError(err)
          ? err.message
          : "Einladung konnte nicht gesendet werden.";
        setError(message);
        logger.error("create_invitation_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      } finally {
        setIsSubmitting(false);
      }
    },
    [email, displayName, isSubmitting, onCreated, toastSuccess],
  );

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
      <h2 className="mb-4 text-lg font-semibold text-gray-900">
        Neuen Operator einladen
      </h2>
      <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
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
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="operator@example.com"
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-violet-500 focus:ring-1 focus:ring-violet-500 focus:outline-none"
            />
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
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="Max Mustermann"
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-violet-500 focus:ring-1 focus:ring-violet-500 focus:outline-none"
            />
          </div>
        </div>

        {error && <p className="text-sm text-red-600">{error}</p>}

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
  const [error, setError] = useState<string | null>(null);
  const { success: toastSuccess } = useToast();

  const handleResend = useCallback(
    async (id: string) => {
      setError(null);
      setActionLoading(id);
      try {
        await resendOperatorInvitation(id);
        toastSuccess("Einladung wurde erneut gesendet.");
        onMutate();
      } catch (err) {
        setError(
          isOperatorApiError(err)
            ? err.message
            : "Einladung konnte nicht erneut gesendet werden.",
        );
      } finally {
        setActionLoading(null);
      }
    },
    [onMutate, toastSuccess],
  );

  const handleRevoke = useCallback(async () => {
    if (!revokeTarget) return;
    setError(null);
    setActionLoading(revokeTarget.id);
    try {
      await revokeOperatorInvitation(revokeTarget.id);
      toastSuccess("Einladung wurde widerrufen.");
      setRevokeTarget(null);
      onMutate();
    } catch (err) {
      setError(
        isOperatorApiError(err)
          ? err.message
          : "Einladung konnte nicht widerrufen werden.",
      );
    } finally {
      setActionLoading(null);
    }
  }, [revokeTarget, onMutate, toastSuccess]);

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
      <h2 className="mb-4 text-lg font-semibold text-gray-900">
        Offene Einladungen
      </h2>

      {error && <p className="mb-4 text-sm text-red-600">{error}</p>}

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
                className="rounded-full bg-violet-100 px-2 py-0.5 text-xs font-medium text-violet-700 transition-colors hover:bg-violet-200 disabled:opacity-50"
              >
                Erneut senden
              </button>
              <button
                type="button"
                disabled={actionLoading === inv.id}
                onClick={() => setRevokeTarget(inv)}
                className="rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700 transition-colors hover:bg-red-200 disabled:opacity-50"
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
        confirmButtonClass="bg-red-500 hover:bg-red-600"
      >
        <p>
          Möchtest du die Einladung an{" "}
          <strong>{revokeTarget?.email ?? ""}</strong> wirklich widerrufen?
        </p>
      </ConfirmationModal>
    </div>
  );
}

// --- Existing Operators List ---

function OperatorsList({ operators }: { readonly operators: OperatorInfo[] }) {
  return (
    <div className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
      <h2 className="mb-4 text-lg font-semibold text-gray-900">
        Bestehende Operatoren
      </h2>

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
                <span
                  className={`mt-1 inline-block rounded-full px-2 py-0.5 text-xs font-medium ${
                    op.active
                      ? "bg-green-100 text-green-700"
                      : "bg-gray-100 text-gray-500"
                  }`}
                >
                  {op.active ? "Aktiv" : "Inaktiv"}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

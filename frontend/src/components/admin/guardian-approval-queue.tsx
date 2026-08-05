"use client";

import { useCallback, useEffect, useState } from "react";
import { Check, X, Settings } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import {
  listPendingApprovals,
  approveGuardianInvitation,
  rejectGuardianInvitation,
  type PendingApproval,
} from "@/lib/guardian-api";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { Loading } from "~/components/ui/loading";
import { ConfirmationModal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { useTenantRouter } from "~/lib/tenant-router";

const logger = createLogger({ component: "GuardianApprovalQueue" });

/** Values of the `guardians.parent_invite_mode` setting. */
export type GuardianInviteMode = "disabled" | "direct" | "staff_approval";

export type GuardianInviteModeState =
  | { readonly status: "loading" }
  | {
      readonly status: "error";
      readonly isRetrying: boolean;
      readonly retry: () => void;
    }
  | { readonly status: "ready"; readonly mode: GuardianInviteMode };

// Only staff_approval ever puts a request into this queue: "disabled" blocks
// parent invites outright and "direct" sends them without a review step. An
// empty list then means "configured away", not "nothing to do yet", so the
// empty state says which of the two it is instead of leaving an admin to
// wonder whether the page is broken. Loading and error states are handled
// separately, so this copy always receives a validated mode.
function emptyStateCopy(inviteMode: GuardianInviteMode): {
  readonly configuredAway: boolean;
  readonly title: string;
  readonly description: string;
} {
  if (inviteMode === "disabled") {
    return {
      configuredAway: true,
      title: "Eltern können derzeit niemanden einladen",
      description:
        "Das Einladen weiterer Bezugspersonen durch Eltern ist ausgeschaltet. Anfragen erscheinen hier erst, wenn die Einstellung auf „Mit Freigabe durch das Team“ steht.",
    };
  }
  if (inviteMode === "direct") {
    return {
      configuredAway: true,
      title: "Einladungen gehen ohne Freigabe raus",
      description:
        "Eltern laden weitere Bezugspersonen aktuell direkt ein, ohne Bestätigung durch das Team. Diese Liste bleibt deshalb leer.",
    };
  }
  return {
    configuredAway: false,
    title: "Keine offenen Anfragen",
    description:
      "Lädt eine Familie im Elternportal eine weitere Bezugsperson zu ihrem Kind ein, erscheint die Anfrage hier zur Freigabe. Bis dahin bekommt niemand zusätzlichen Zugang.",
  };
}

function ApprovalsEmptyState({
  inviteMode,
}: {
  readonly inviteMode: GuardianInviteMode;
}) {
  const router = useTenantRouter();
  const { configuredAway, title, description } = emptyStateCopy(inviteMode);
  const iconColor = configuredAway
    ? LOCATION_COLORS.OTHER_ROOM
    : LOCATION_COLORS.GROUP_ROOM;
  const iconBackground = `${iconColor}${configuredAway ? "1F" : "24"}`;

  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <EmptyState
        icon={
          <span
            className="flex h-12 w-12 items-center justify-center rounded-2xl"
            style={{ backgroundColor: iconBackground, color: iconColor }}
          >
            {configuredAway ? (
              <Settings className="h-6 w-6" />
            ) : (
              <MotoConceptIcon concept="accounts" size={28} />
            )}
          </span>
        }
        title={title}
        description={description}
        action={
          configuredAway ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => router.push("/settings?tab=operations")}
            >
              Zu den Einstellungen
            </Button>
          ) : undefined
        }
      />
    </div>
  );
}

function InviteModeDependentEmptyState({
  state,
}: {
  readonly state: GuardianInviteModeState;
}) {
  if (state.status === "loading") {
    return (
      <Loading
        message="Einladungs-Einstellung wird geladen…"
        fullPage={false}
      />
    );
  }

  if (state.status === "error") {
    return (
      <Alert
        type="error"
        message="Die Einladungs-Einstellung konnte nicht geladen werden. Der leere Zustand der Konto-Anfragen kann deshalb derzeit nicht zuverlässig eingeordnet werden."
        action={
          <Button
            type="button"
            variant="outline_danger"
            size="md"
            isLoading={state.isRetrying}
            loadingText="Wird geladen…"
            onClick={state.retry}
          >
            Erneut versuchen
          </Button>
        }
      />
    );
  }

  return <ApprovalsEmptyState inviteMode={state.mode} />;
}

export default function GuardianApprovalQueue({
  inviteModeState,
}: {
  readonly inviteModeState: GuardianInviteModeState;
}) {
  const [requests, setRequests] = useState<PendingApproval[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actingId, setActingId] = useState<string | null>(null);
  const [rejectTarget, setRejectTarget] = useState<PendingApproval | null>(
    null,
  );
  const { success: toastSuccess, error: toastError } = useToast();

  const load = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      setRequests(await listPendingApprovals());
    } catch (err) {
      logger.error("approvals_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError("Anfragen konnten nicht geladen werden");
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const handleApprove = async (req: PendingApproval) => {
    setActingId(req.id);
    try {
      await approveGuardianInvitation(req.id);
      await load();
      toastSuccess(
        `${req.guardianName || req.guardianEmail || "Bezugsperson"} wurde freigegeben`,
      );
    } catch (err) {
      logger.error("approval_approve_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toastError("Freigabe fehlgeschlagen");
    } finally {
      setActingId(null);
    }
  };

  const handleReject = async () => {
    if (!rejectTarget) return;
    const req = rejectTarget;
    setActingId(req.id);
    try {
      await rejectGuardianInvitation(req.id);
      setRejectTarget(null);
      await load();
      toastSuccess("Anfrage abgelehnt");
    } catch (err) {
      logger.error("approval_reject_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toastError("Ablehnen fehlgeschlagen");
    } finally {
      setActingId(null);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center rounded-2xl border border-gray-200 bg-white p-12 shadow-sm">
        <div className="flex items-center gap-3 text-sm text-gray-600">
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-gray-200 border-t-gray-900" />
          Wird geladen…
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {error && (
        <div className="rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700">
          {error}
          <button
            type="button"
            onClick={() => void load()}
            className="ml-2 rounded-lg bg-red-100 px-2 py-1 text-xs font-medium text-red-700 hover:bg-red-200"
          >
            Erneut versuchen
          </button>
        </div>
      )}

      {requests.length === 0 && !error ? (
        <InviteModeDependentEmptyState state={inviteModeState} />
      ) : (
        requests.map((req) => (
          <div
            key={req.id}
            className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm"
          >
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="min-w-0">
                <p className="truncate text-base font-semibold text-gray-900">
                  {req.guardianName || req.guardianEmail || "Bezugsperson"}
                </p>
                {req.guardianEmail && (
                  <p className="truncate text-sm text-gray-600">
                    {req.guardianEmail}
                  </p>
                )}
                <p className="mt-1 text-xs text-gray-500">
                  {req.studentName ? (
                    <>
                      Zugang zu{" "}
                      <span className="font-medium text-gray-700">
                        {req.studentName}
                      </span>
                    </>
                  ) : (
                    "Zugang zu einem Kind"
                  )}
                  {req.requestedByEmail && (
                    <> · angefragt von {req.requestedByEmail}</>
                  )}
                </p>
              </div>
              <div className="flex flex-shrink-0 items-center gap-2">
                <Button
                  type="button"
                  variant="success"
                  size="sm"
                  isLoading={actingId === req.id}
                  onClick={() => void handleApprove(req)}
                >
                  <Check className="mr-1 h-4 w-4" />
                  Freigeben
                </Button>
                <Button
                  type="button"
                  variant="outline_danger"
                  size="sm"
                  disabled={actingId === req.id}
                  onClick={() => setRejectTarget(req)}
                >
                  <X className="mr-1 h-4 w-4" />
                  Ablehnen
                </Button>
              </div>
            </div>
          </div>
        ))
      )}

      <ConfirmationModal
        isOpen={!!rejectTarget}
        onClose={() => setRejectTarget(null)}
        onConfirm={() => void handleReject()}
        title="Anfrage ablehnen?"
        confirmText="Ablehnen"
        cancelText="Abbrechen"
        confirmButtonClass="bg-red-600 hover:bg-red-700"
      >
        <p className="text-sm text-gray-600">
          Möchtest du die Anfrage für{" "}
          <span className="font-medium text-gray-900">
            {rejectTarget?.guardianName ||
              rejectTarget?.guardianEmail ||
              "diese Bezugsperson"}
          </span>{" "}
          ablehnen? Es wird kein Zugang gewährt.
        </p>
      </ConfirmationModal>
    </div>
  );
}

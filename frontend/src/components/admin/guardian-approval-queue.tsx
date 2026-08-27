"use client";

import { useCallback, useEffect, useState } from "react";
import { Check, X } from "lucide-react";
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
import { ConfirmationModal } from "~/components/ui/modal";
import { CardSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
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

// "disabled" blocks parent invites outright and "direct" sends NEW invites
// without a review step — but a parent-confirmed upgrade of an existing
// restricted contact is always queued here, regardless of mode (#2172). An
// empty list in the other two modes therefore mostly means "configured
// away", so the empty state says which of the two it is instead of leaving
// an admin to wonder whether the page is broken. Loading and error states
// are handled separately, so this copy always receives a validated mode.
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
        "Eltern laden neue Bezugspersonen aktuell direkt ein, ohne Bestätigung durch das Team. Nur wenn Eltern einen bestehenden, eingeschränkten Kontakt auf vollen Zugriff hochstufen möchten, erscheint die Anfrage hier zur Freigabe.",
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

  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <EmptyState
        icon={
          <span
            data-testid="approvals-empty-icon"
            data-concept={configuredAway ? "settings" : "accounts"}
            className="flex h-12 w-12 items-center justify-center rounded-2xl bg-gray-100"
          >
            <MotoConceptIcon
              concept={configuredAway ? "settings" : "accounts"}
              size={28}
            />
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
      <SkeletonRegion label="Einladungs-Einstellung wird geladen…">
        <CardSkeleton rows={2} />
      </SkeletonRegion>
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
  onCountChange,
}: {
  readonly inviteModeState: GuardianInviteModeState;
  /** Meldet die Zahl offener Anfragen an den Seitenkopf; `null` = am Laden. */
  readonly onCountChange?: (count: number | null) => void;
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

  const reportedCount = isLoading ? null : requests.length;
  useEffect(() => {
    onCountChange?.(reportedCount);
  }, [onCountChange, reportedCount]);

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
      <SkeletonRegion label="Konto-Anfragen werden geladen">
        <CardSkeleton rows={3} />
      </SkeletonRegion>
    );
  }

  return (
    <div className="space-y-3">
      {error ? (
        <Alert
          type="error"
          message={error}
          action={
            <Button
              type="button"
              variant="outline_danger"
              size="md"
              onClick={() => void load()}
            >
              Erneut versuchen
            </Button>
          }
        />
      ) : null}

      {requests.length === 0 && !error ? (
        <InviteModeDependentEmptyState state={inviteModeState} />
      ) : (
        requests.map((req) => (
          <div
            key={req.id}
            className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6"
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
                {req.roleUpgrade && (
                  <p className="mt-1 text-xs text-gray-500">
                    Stuft einen bestehenden Kontakt auf vollen Zugriff hoch.
                  </p>
                )}
              </div>
              <div className="flex flex-shrink-0 items-center gap-2">
                <Button
                  type="button"
                  variant="success"
                  size="md"
                  isLoading={actingId === req.id}
                  onClick={() => void handleApprove(req)}
                >
                  <Check className="mr-1 h-4 w-4" />
                  Freigeben
                </Button>
                <Button
                  type="button"
                  variant="outline_danger"
                  size="md"
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
        confirmButtonClass="bg-moto-red hover:bg-moto-red-hover text-white"
      >
        <p className="text-sm text-gray-600">
          Möchten Sie die Anfrage für{" "}
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

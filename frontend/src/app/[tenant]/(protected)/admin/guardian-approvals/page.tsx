"use client";

import { useState } from "react";

import GuardianApprovalQueue, {
  type GuardianInviteModeState,
} from "~/components/admin/guardian-approval-queue";
import { TenantPage } from "~/components/ui/tenant-page";
import { canReviewGuardianApprovals } from "~/lib/guardian-approval-access";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";
import { useSchoolParentInviteMode } from "~/lib/hooks/use-school-switches";
import { USER_CONTEXT_SWR_KEY } from "~/lib/hooks/use-user-context";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useTenantMutate } from "~/lib/swr";

// Staff approval queue for parent-initiated guardian invitations. Only
// relevant when a school sets guardians.parent_invite_mode = staff_approval;
// otherwise the queue is simply empty.
export default function GuardianApprovalsPage() {
  // Lesen verlangt users:manage, Annehmen und Ablehnen users:update;
  // dieselbe Regel zeigt den Eintrag in der Elterngruppe (#3469).
  const { isReady } = useRequirePermission(canReviewGuardianApprovals);
  // Zeilenzahl der Warteschlange, von ihr selbst gemeldet (kein zusätzlicher
  // Request). `null` = noch am Laden.
  const [pendingCount, setPendingCount] = useState<number | null>(null);
  // The queue is empty by design in the other two invite modes, so the empty
  // state needs to know which one is active. The mode rides on the signed-in
  // navigation context (#4996), which the tenant layout usually preloads; the
  // queue still mounts while it loads. A failed request or an unknown mode
  // must not make disabled/direct mode look like an empty staff-approval
  // queue.
  const tenantMutate = useTenantMutate();
  const {
    mode: inviteMode,
    isLoading: isModeLoading,
    error: modeError,
  } = useSchoolParentInviteMode(isReady);
  const retryMode = () => void tenantMutate(USER_CONTEXT_SWR_KEY);
  // A failed request goes through the shared error path (#2517): catalog
  // text, retry and request ID in the queue's empty area.
  const modeLoadError = useSwrLoadError(
    modeError,
    "die Einladungs-Einstellung",
    retryMode,
  );

  const inviteModeState: GuardianInviteModeState =
    !isReady || isModeLoading
      ? { status: "loading" }
      : modeError != null || inviteMode === undefined
        ? {
            status: "error",
            isRetrying: false,
            retry: retryMode,
            error: modeLoadError,
            awaitingError: modeError != null && modeLoadError === null,
          }
        : { status: "ready", mode: inviteMode };

  return (
    <TenantPage
      title="Elternzugänge"
      stats={pendingCount === null ? null : `${pendingCount} offen`}
      statsLoading={pendingCount === null}
      loading={!isReady}
    >
      <GuardianApprovalQueue
        inviteModeState={inviteModeState}
        onCountChange={setPendingCount}
      />
    </TenantPage>
  );
}

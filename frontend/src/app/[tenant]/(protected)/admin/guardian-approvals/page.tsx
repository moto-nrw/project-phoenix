"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

import GuardianApprovalQueue, {
  type GuardianInviteModeState,
} from "~/components/admin/guardian-approval-queue";
import { TenantPage } from "~/components/ui/tenant-page";
import { canReviewGuardianApprovals } from "~/lib/guardian-approval-access";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";
import { useParentInviteMode } from "~/lib/tenant-context";

// Staff approval queue for parent-initiated guardian invitations. Only
// relevant when a school sets guardians.parent_invite_mode = staff_approval;
// otherwise the queue is simply empty.
export default function GuardianApprovalsPage() {
  // Lesen verlangt users:manage, Annehmen und Ablehnen users:update;
  // dieselbe Regel zeigt den Eintrag in der Elterngruppe (#3469).
  const { isReady } = useRequirePermission(canReviewGuardianApprovals);
  const router = useRouter();
  // Zeilenzahl der Warteschlange, von ihr selbst gemeldet (kein zusätzlicher
  // Request). `null` = noch am Laden.
  const [pendingCount, setPendingCount] = useState<number | null>(null);
  // The queue is empty by design in the other two invite modes, so the empty
  // state needs to know which one is active. The mode rides on the tenant
  // resolve (#4996) instead of the whole settings schema. A missing or unknown
  // mode must not make disabled/direct mode look like an empty
  // staff-approval queue; a reload resolves the school again.
  const inviteMode = useParentInviteMode();

  const inviteModeState: GuardianInviteModeState =
    inviteMode === undefined
      ? { status: "error", isRetrying: false, retry: () => router.refresh() }
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

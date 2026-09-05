"use client";

import { useState } from "react";

import GuardianApprovalQueue, {
  type GuardianInviteMode,
  type GuardianInviteModeState,
} from "~/components/admin/guardian-approval-queue";
import { TenantPage } from "~/components/ui/tenant-page";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { getSettingValue } from "~/lib/settings-api";

const INVITE_MODES: readonly GuardianInviteMode[] = [
  "disabled",
  "direct",
  "staff_approval",
];

function parseInviteMode(value: unknown): GuardianInviteMode | undefined {
  return INVITE_MODES.find((mode) => mode === value);
}

// Staff approval queue for parent-initiated guardian invitations. Only
// relevant when a school sets guardians.parent_invite_mode = staff_approval;
// otherwise the queue is simply empty.
export default function GuardianApprovalsPage() {
  const { isReady } = useRequireAdmin();
  // Zeilenzahl der Warteschlange, von ihr selbst gemeldet (kein zusätzlicher
  // Request). `null` = noch am Laden.
  const [pendingCount, setPendingCount] = useState<number | null>(null);
  // The queue is empty by design in the other two invite modes, so the empty
  // state needs to know which one is active. The queue still mounts while this
  // independent request runs; only its empty-result copy waits for the mode.
  // A failed or incomplete schema must not make disabled/direct mode look like
  // an empty staff-approval queue.
  const {
    data: settingsSchema,
    error: settingsError,
    isLoading: isSettingsLoading,
    isValidating: isSettingsValidating,
    mutate: retrySettings,
  } = useSettingsSchema(isReady, {
    revalidateOnFocus: false,
    revalidateOnReconnect: false,
    shouldRetryOnError: false,
  });
  const inviteMode = parseInviteMode(
    getSettingValue(settingsSchema, "guardians.parent_invite_mode"),
  );

  const inviteModeState: GuardianInviteModeState = isSettingsLoading
    ? { status: "loading" }
    : settingsError != null ||
        settingsSchema == null ||
        inviteMode === undefined
      ? {
          status: "error",
          isRetrying: isSettingsValidating,
          retry: () => void retrySettings(),
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

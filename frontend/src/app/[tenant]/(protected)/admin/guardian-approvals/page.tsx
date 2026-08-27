"use client";

import GuardianApprovalQueue, {
  type GuardianInviteMode,
  type GuardianInviteModeState,
} from "~/components/admin/guardian-approval-queue";
import { PageIntro } from "~/components/ui/page-intro";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
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
    <div className="w-full space-y-6">
      {/* Kopfkarte wie in der Eltern-App: Kicker, Titel und Erklärtext in EINER
          Karte, auf allen Breakpoints. Die Seite hat weder Suche noch Filter,
          deshalb entfällt PageHeaderWithSearch ganz. */}
      <PageIntro
        kicker="Eltern"
        title="Konto-Anfragen"
        description="Zugänge von Eltern zum Elternportal prüfen und freigeben."
      />
      <div>
        {isReady ? (
          <GuardianApprovalQueue inviteModeState={inviteModeState} />
        ) : (
          <SkeletonRegion label="Konto-Anfragen werden geladen">
            <ListSkeleton rows={5} />
          </SkeletonRegion>
        )}
      </div>
    </div>
  );
}

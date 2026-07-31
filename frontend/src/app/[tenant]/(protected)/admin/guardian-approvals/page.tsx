"use client";

import GuardianApprovalQueue, {
  type GuardianInviteMode,
} from "~/components/admin/guardian-approval-queue";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { Loading } from "~/components/ui/loading";
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
  // state needs to know which one is active. Admins hold config:read; if the
  // schema fetch fails the mode stays undefined and the empty state falls back
  // to neutral wording.
  const { data: settingsSchema } = useSettingsSchema(isReady, {
    revalidateOnFocus: false,
    revalidateOnReconnect: false,
    shouldRetryOnError: false,
  });
  const inviteMode = parseInviteMode(
    getSettingValue(settingsSchema, "guardians.parent_invite_mode"),
  );

  if (!isReady) return <Loading fullPage={false} />;

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Konto-Anfragen" />
      <div className="mt-4">
        <GuardianApprovalQueue inviteMode={inviteMode} />
      </div>
    </div>
  );
}

"use client";

import GuardianApprovalQueue from "~/components/admin/guardian-approval-queue";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { Loading } from "~/components/ui/loading";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

// Staff approval queue for parent-initiated guardian invitations. Only
// relevant when a school sets guardians.parent_invite_mode = staff_approval;
// otherwise the queue is simply empty.
export default function GuardianApprovalsPage() {
  const { isReady } = useRequireAdmin();
  if (!isReady) return <Loading fullPage={false} />;

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Konto-Anfragen" />
      <div className="mt-4">
        <GuardianApprovalQueue />
      </div>
    </div>
  );
}

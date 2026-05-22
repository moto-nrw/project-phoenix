"use client";

import { use } from "react";
import { AdminEnrollmentPhaseDetail } from "~/components/enrollment/admin-enrollment-phase-detail";
import { Loading } from "~/components/ui/loading";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

interface PageProps {
  readonly params: Promise<{ tenant: string; phaseId: string }>;
}

export default function AdminEnrollmentPhasePage({ params }: PageProps) {
  const { phaseId } = use(params);
  const { isReady } = useRequireAdmin();
  if (!isReady) return <Loading fullPage={false} />;

  return (
    <div className="w-full">
      <MobileBackButton />
      <AdminEnrollmentPhaseDetail phaseId={phaseId} />
    </div>
  );
}

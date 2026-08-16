"use client";

import { use } from "react";
import { AdminEnrollmentPhaseDetail } from "~/components/enrollment/admin-enrollment-phase-detail";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import { DetailSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { Skeleton } from "~/components/ui/skeleton";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

interface PageProps {
  readonly params: Promise<{ tenant: string; phaseId: string }>;
}

export default function AdminEnrollmentPhasePage({ params }: PageProps) {
  const { phaseId } = use(params);
  const { isReady } = useRequireAdmin();
  if (!isReady)
    return (
      <SkeletonRegion label="Anmeldephase wird geladen" className="space-y-4">
        <Skeleton className="h-6 w-56 rounded" />
        <DetailSkeleton sections={2} fieldsPerSection={4} />
      </SkeletonRegion>
    );

  return (
    <div className="w-full">
      <MobileBackButton />
      <AdminEnrollmentPhaseDetail phaseId={phaseId} />
    </div>
  );
}

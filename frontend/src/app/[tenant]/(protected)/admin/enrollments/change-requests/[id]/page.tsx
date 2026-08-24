"use client";

import { use } from "react";
import { AdminEnrollmentChangeRequestDetail } from "~/components/enrollment/admin-enrollment-change-requests";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import { DetailSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { Skeleton } from "~/components/ui/skeleton";
import { canReviewEnrollmentChangeRequests } from "~/lib/change-request-access";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";

interface PageProps {
  readonly params: Promise<{ tenant: string; id: string }>;
}

export default function AdminEnrollmentChangeRequestDetailPage({
  params,
}: PageProps) {
  const { id } = use(params);
  const { isReady } = useRequirePermission(canReviewEnrollmentChangeRequests);
  if (!isReady)
    return (
      <SkeletonRegion
        label="Änderungsanfrage wird geladen"
        className="space-y-4"
      >
        <Skeleton className="h-6 w-56 rounded" />
        <DetailSkeleton sections={2} fieldsPerSection={4} />
      </SkeletonRegion>
    );

  return (
    <div className="w-full">
      <MobileBackButton />
      <AdminEnrollmentChangeRequestDetail changeRequestId={id} />
    </div>
  );
}

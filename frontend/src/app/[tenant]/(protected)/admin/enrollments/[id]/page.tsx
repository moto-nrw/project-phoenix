"use client";

import { use } from "react";
import { AdminEnrollmentDetail } from "~/components/enrollment/admin-enrollment-detail";
import { BackButton } from "~/components/ui/back-button";
import { DetailSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { Skeleton } from "~/components/ui/skeleton";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

interface PageProps {
  readonly params: Promise<{ tenant: string; id: string }>;
}

export default function AdminEnrollmentDetailPage({ params }: PageProps) {
  const { id } = use(params);
  const { isReady } = useRequireAdmin();
  if (!isReady)
    return (
      <SkeletonRegion label="Anmeldung wird geladen" className="space-y-4">
        <div className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
          <Skeleton className="h-3 w-24 rounded" />
          <Skeleton className="mt-2 h-6 w-64 rounded" />
          <Skeleton className="mt-2 h-4 w-80 rounded" />
        </div>
        <DetailSkeleton sections={3} fieldsPerSection={4} />
      </SkeletonRegion>
    );

  return (
    <div className="w-full">
      <BackButton referrer="/admin/enrollments" />
      <AdminEnrollmentDetail requestId={id} />
    </div>
  );
}

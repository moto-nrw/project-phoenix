"use client";

import { use } from "react";
import { AdminEnrollmentChangeRequestDetail } from "~/components/enrollment/admin-enrollment-change-requests";
import { Loading } from "~/components/ui/loading";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

interface PageProps {
  readonly params: Promise<{ tenant: string; id: string }>;
}

export default function AdminEnrollmentChangeRequestDetailPage({
  params,
}: PageProps) {
  const { id } = use(params);
  const { isReady } = useRequireAdmin();
  if (!isReady) return <Loading fullPage={false} />;

  return (
    <div className="w-full">
      <MobileBackButton />
      <AdminEnrollmentChangeRequestDetail changeRequestId={id} />
    </div>
  );
}

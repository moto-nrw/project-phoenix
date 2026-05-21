"use client";

import { AdminEnrollmentsList } from "~/components/enrollment/admin-enrollments-list";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { Loading } from "~/components/ui/loading";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function AdminEnrollmentsPage() {
  const { isReady } = useRequireAdmin();
  if (!isReady) return <Loading fullPage={false} />;

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Prozess" />
      <AdminEnrollmentsList />
    </div>
  );
}

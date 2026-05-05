"use client";

import { use } from "react";
import { AdminEnrollmentDetail } from "~/components/enrollment/admin-enrollment-detail";

interface PageProps {
  readonly params: Promise<{ tenant: string; id: string }>;
}

export default function AdminEnrollmentDetailPage({ params }: PageProps) {
  const { id } = use(params);
  return (
    <div className="mx-auto max-w-4xl space-y-6 p-6">
      <AdminEnrollmentDetail requestId={id} />
    </div>
  );
}

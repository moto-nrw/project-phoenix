"use client";

import { use } from "react";
import { AdminEnrollmentDetail } from "~/components/enrollment/admin-enrollment-detail";
import { MobileBackButton } from "~/components/ui/mobile-back-button";

interface PageProps {
  readonly params: Promise<{ tenant: string; id: string }>;
}

export default function AdminEnrollmentDetailPage({ params }: PageProps) {
  const { id } = use(params);
  return (
    <div className="w-full max-w-5xl">
      <MobileBackButton />
      <AdminEnrollmentDetail requestId={id} />
    </div>
  );
}

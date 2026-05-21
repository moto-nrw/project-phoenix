"use client";

import { use } from "react";
import { useSearchParams } from "next/navigation";
import { EnrollmentStatusView } from "~/components/enrollment/enrollment-status-view";
import {
  PublicEnrollmentBrand,
  PublicEnrollmentPageShell,
  PublicEnrollmentSteps,
} from "~/components/enrollment/public-enrollment-shell";
import { useTenant } from "~/components/tenant/tenant-provider";

interface PageProps {
  readonly params: Promise<{ tenant: string; token: string }>;
}

export default function EnrollmentStatusPage({ params }: PageProps) {
  const { token } = use(params);
  const searchParams = useSearchParams();
  const { tenant } = useTenant();
  const justSubmitted = searchParams.get("submitted") === "1";

  return (
    <PublicEnrollmentPageShell>
      <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <PublicEnrollmentBrand tenant={tenant} />
        <PublicEnrollmentSteps current="done" />
      </div>
      <EnrollmentStatusView token={token} justSubmitted={justSubmitted} />
    </PublicEnrollmentPageShell>
  );
}

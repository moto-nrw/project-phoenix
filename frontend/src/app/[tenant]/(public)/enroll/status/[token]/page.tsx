"use client";

import { Suspense, use } from "react";
import { useSearchParams } from "next/navigation";
import { EnrollmentStatusView } from "~/components/enrollment/enrollment-status-view";
import {
  PublicEnrollmentBrand,
  PublicEnrollmentLocaleSwitcher,
  PublicEnrollmentPageShell,
  PublicEnrollmentSteps,
} from "~/components/enrollment/public-enrollment-shell";
import { useTenant } from "~/lib/tenant-context";

interface PageProps {
  readonly params: Promise<{ tenant: string; token: string }>;
}

export default function EnrollmentStatusPage({ params }: PageProps) {
  return (
    <Suspense fallback={null}>
      <EnrollmentStatusPageContent params={params} />
    </Suspense>
  );
}

function EnrollmentStatusPageContent({ params }: PageProps) {
  const { token } = use(params);
  const searchParams = useSearchParams();
  const { tenant } = useTenant();
  const justSubmitted = searchParams.get("submitted") === "1";
  const duplicateWarning = searchParams.get("duplicate_warning") === "1";

  return (
    <PublicEnrollmentPageShell withInlineSwitcher>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4 sm:mb-8">
        <PublicEnrollmentBrand tenant={tenant} />
        <div className="flex items-center gap-3">
          <PublicEnrollmentSteps current="done" />
          <PublicEnrollmentLocaleSwitcher />
        </div>
      </div>
      <EnrollmentStatusView
        token={token}
        justSubmitted={justSubmitted}
        duplicateWarning={duplicateWarning}
      />
    </PublicEnrollmentPageShell>
  );
}
